package genui

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

const (
	CatalogID         = "markgogo.dev:genui/v1"
	TargetA2UIVersion = "1.0"
	ViewVersion       = 2
	MaxGlobalDepth    = 50
	MaxFuncCallDepth  = 5
)

var catalogComponentNames = []string{
	"Column",
	"Row",
	"Tabs",
	"Card",
	"Text",
	"KeyPoints",
	"MetricGrid",
	"ComparisonTable",
	"Timeline",
	"Checklist",
}

var knownCatalogComponents = map[string]bool{
	"Column":          true,
	"Row":             true,
	"Tabs":            true,
	"Card":            true,
	"Text":            true,
	"KeyPoints":       true,
	"MetricGrid":      true,
	"ComparisonTable": true,
	"Timeline":        true,
	"Checklist":       true,
}

var (
	rowAllowedParents   = []string{"Surface", "Column", "Tabs"}
	tabsAllowedParents  = []string{"Surface", "Column"}
	cardAllowedChildren = []string{"Column", "Text", "KeyPoints", "MetricGrid", "ComparisonTable", "Timeline", "Checklist"}
)

var nativeActionKeys = []string{
	"agentFunctionResponse",
	"callRendererFunction",
	"createSurface",
	"deleteSurface",
	"updateComponents",
	"updateDataModel",
}

func CatalogComponents() []string {
	return append([]string(nil), catalogComponentNames...)
}

type ValidationError struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Path        string `json:"path,omitempty"`
	ComponentID string `json:"componentId,omitempty"`
	Hint        string `json:"hint,omitempty"`
}

func (e *ValidationError) Error() string {
	return e.Message
}

func newValidationError(code, message, path, componentID string) *ValidationError {
	return &ValidationError{
		Code:        code,
		Message:     message,
		Path:        path,
		ComponentID: componentID,
		Hint:        HintInvalidOutput,
	}
}

func ValidateCandidate(ctx context.Context, candidate []byte) error {
	if ctx != nil && ctx.Err() != nil {
		return ErrCancelled
	}
	if err := ValidateGeneratedView(candidate); err != nil {
		return NewFailure(FailInvalidOutput, "The generated view is not valid: "+err.Error(), HintInvalidOutput)
	}
	return nil
}

func ValidationUnavailable(detail string) error {
	detail = strings.TrimSuffix(strings.TrimSpace(detail), ".")
	return NewFailure(FailInvalidOutput, "The generated view could not be checked, so it was not saved: "+detail+".", HintInvalidOutput)
}

func ValidateGeneratedView(raw []byte) error {
	return validateView(raw, false)
}

func ValidateStoredView(raw []byte) error {
	return validateView(raw, true)
}

func validateView(raw []byte, stored bool) error {
	var top any
	if err := json.Unmarshal(raw, &top); err != nil {
		return newValidationError("INVALID_JSON", "The view is not valid JSON.", "/", "")
	}
	file, ok := top.(map[string]any)
	if !ok {
		return newValidationError("INVALID_ENVELOPE", "The view is not a JSON object.", "/", "")
	}
	ver, ok := file["version"].(float64)
	if !ok || ver != ViewVersion {
		return newValidationError("INVALID_VERSION", "The view is not a version 2 view file.", "/version", "")
	}
	sourceValue, present := file["sources"]
	rawSources, ok := sourceValue.(map[string]any)
	if !ok && (present || !stored) {
		return newValidationError("INVALID_SOURCES", "The view's sources are not an object.", "/sources", "")
	}
	if err := validateSourcesObject(rawSources); err != nil {
		return err
	}
	rawMessages, ok := file["a2ui"].([]any)
	if !ok || len(rawMessages) == 0 {
		return newValidationError("EMPTY_MESSAGES", "The view has no A2UI messages.", "/a2ui", "")
	}
	return validateA2UIMessages(rawMessages, rawSources, stored)
}

func validateSourcesObject(sources map[string]any) error {
	keys := sortedKeys(sources)
	for _, id := range keys {
		entryVal := sources[id]
		entry, ok := entryVal.(map[string]any)
		if !ok {
			return newValidationError("INVALID_SOURCES", fmt.Sprintf("Source %q needs a \"heading\" string.", id), "/sources/"+id, "")
		}
		heading, ok := entry["heading"].(string)
		if !ok || strings.TrimSpace(heading) == "" {
			return newValidationError("INVALID_SOURCES", fmt.Sprintf("Source %q needs a \"heading\" string.", id), "/sources/"+id+"/heading", "")
		}
		if occVal, exists := entry["occurrence"]; exists && occVal != nil {
			occFloat, ok := occVal.(float64)
			if !ok || occFloat < 1 || math.Trunc(occFloat) != occFloat {
				return newValidationError("INVALID_SOURCES", fmt.Sprintf("Source %q: \"occurrence\" must be a whole number from 1.", id), "/sources/"+id+"/occurrence", "")
			}
		}
		if hashVal, exists := entry["hash"]; exists {
			if _, ok := hashVal.(string); !ok {
				return newValidationError("INVALID_SOURCES", fmt.Sprintf("Source %q: \"hash\" must be a string.", id), "/sources/"+id+"/hash", "")
			}
		}
	}
	return nil
}

func validateA2UIMessages(messages []any, sources map[string]any, stored bool) error {
	if err := checkUnknownComponents(messages, stored); err != nil {
		return err
	}

	allowOrphans := stored
	for _, m := range messages {
		if obj, ok := m.(map[string]any); ok {
			if _, ok := obj["updateComponents"].(map[string]any); ok {
				allowOrphans = true
				break
			}
		}
	}

	type parsedOp struct {
		kind          string
		msgIdx        int
		surfaceID     string
		catalogID     string
		hasCatalogID  bool
		components    []map[string]any
		hasComponents bool
	}
	ops := make([]parsedOp, 0, len(messages))

	for i, rawMsg := range messages {
		msgPath := fmt.Sprintf("/a2ui/%d", i)
		if err := traverseRecursionAndPaths(rawMsg, 0, 0, msgPath); err != nil {
			return err
		}
		if err := validateTargetVersion(rawMsg, msgPath); err != nil {
			return err
		}
		op, err := extractMessageOperation(rawMsg, i, msgPath)
		if err != nil {
			return err
		}
		ops = append(ops, parsedOp{
			kind:          op.kind,
			msgIdx:        i,
			surfaceID:     op.surfaceID,
			catalogID:     op.catalogID,
			hasCatalogID:  op.hasCatalogID,
			components:    op.components,
			hasComponents: op.hasComponents,
		})
	}

	surfaces := make(map[string]*surfaceState)
	var surfaceOrder []string

	for _, op := range ops {
		msgPath := fmt.Sprintf("/a2ui/%d", op.msgIdx)
		if op.kind != "createSurface" && op.kind != "updateComponents" {
			return newValidationError(
				"UNSUPPORTED_MESSAGE",
				fmt.Sprintf("Operation '%s' is not permitted by ValidationConfig.allowedMessages", op.kind),
				msgPath,
				"",
			)
		}
		switch op.kind {
		case "createSurface":
			if op.hasCatalogID && op.catalogID != CatalogID {
				return newValidationError(
					"CATALOG_ERROR",
					fmt.Sprintf("Catalog not found: %s", op.catalogID),
					msgPath+"/createSurface/catalogId",
					"",
				)
			}
			if _, exists := surfaces[op.surfaceID]; exists {
				return newValidationError(
					"DUPLICATE_SURFACE",
					fmt.Sprintf("Surface %s already exists.", op.surfaceID),
					msgPath+"/createSurface/surfaceId",
					"",
				)
			}
			s := &surfaceState{
				id:             op.surfaceID,
				defaultCatalog: op.catalogID,
				components:     make(map[string]*componentEntry),
			}
			surfaces[op.surfaceID] = s
			surfaceOrder = append(surfaceOrder, op.surfaceID)
			if len(op.components) > 0 {
				if err := applyUpdateComponents(s, op.components, allowOrphans, msgPath+"/createSurface"); err != nil {
					return err
				}
			}
		case "updateComponents":
			s, exists := surfaces[op.surfaceID]
			if !exists {
				return newValidationError(
					"SURFACE_NOT_FOUND",
					fmt.Sprintf("Surface not found for message: %s", op.surfaceID),
					msgPath+"/updateComponents/surfaceId",
					"",
				)
			}
			if err := applyUpdateComponents(s, op.components, allowOrphans, msgPath+"/updateComponents"); err != nil {
				return err
			}
		}
	}

	if len(surfaceOrder) != 1 {
		return newValidationError(
			"SURFACE_COUNT_ERROR",
			fmt.Sprintf("The view must create exactly one surface; it creates %d.", len(surfaceOrder)),
			"/a2ui",
			"",
		)
	}

	onlySurface := surfaces[surfaceOrder[0]]
	if _, hasRoot := onlySurface.components["root"]; !hasRoot {
		return newValidationError(
			"MISSING_ROOT",
			"Missing root component: No component has id='root'",
			"/a2ui",
			"root",
		)
	}

	if stored {
		return nil
	}
	return checkMissingSources(messages, sources)
}

func checkUnknownComponents(messages []any, stored bool) error {
	for i, rawMsg := range messages {
		msg, ok := rawMsg.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"createSurface", "updateComponents"} {
			body, ok := msg[key].(map[string]any)
			if !ok {
				continue
			}
			comps, ok := body["components"].([]any)
			if !ok {
				continue
			}
			for j, rawComp := range comps {
				comp, ok := rawComp.(map[string]any)
				if !ok {
					continue
				}
				compType, ok := comp["component"].(string)
				if !ok {
					continue
				}
				if knownCatalogComponents[compType] {
					continue
				}
				if stored {
					comps[j] = map[string]any{"id": comp["id"], "component": "Text", "text": ""}
					continue
				}
				id := "(no id)"
				if s, ok := comp["id"].(string); ok {
					id = s
				}
				return newValidationError(
					"UNKNOWN_COMPONENT_TYPE",
					fmt.Sprintf("Component '%s' uses '%s', which is not a component this app can show.", id, compType),
					fmt.Sprintf("/a2ui/%d/%s/components/%d/component", i, key, j),
					id,
				)
			}
		}
	}
	return nil
}

func checkMissingSources(messages []any, sources map[string]any) error {
	for i, rawMsg := range messages {
		msg, ok := rawMsg.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"createSurface", "updateComponents"} {
			body, ok := msg[key].(map[string]any)
			if !ok {
				continue
			}
			comps, ok := body["components"].([]any)
			if !ok {
				continue
			}
			for j, rawComp := range comps {
				comp, ok := rawComp.(map[string]any)
				if !ok {
					continue
				}
				srcList, ok := comp["sources"].([]any)
				if !ok {
					continue
				}
				for k, rawSrc := range srcList {
					src, ok := rawSrc.(string)
					if !ok {
						continue
					}
					if _, exists := sources[src]; !exists {
						id := "(no id)"
						if s, ok := comp["id"].(string); ok {
							id = s
						}
						return newValidationError(
							"MISSING_SOURCE",
							fmt.Sprintf("Component '%s' uses the source '%s', which the view does not have.", id, src),
							fmt.Sprintf("/a2ui/%d/%s/components/%d/sources/%d", i, key, j, k),
							id,
						)
					}
				}
			}
		}
	}
	return nil
}

func traverseRecursionAndPaths(item any, globalDepth, funcDepth int, currentPath string) error {
	if globalDepth > MaxGlobalDepth {
		return newValidationError(
			"MAX_DEPTH_EXCEEDED",
			fmt.Sprintf("Global recursion limit exceeded: Depth > %d", MaxGlobalDepth),
			currentPath,
			"",
		)
	}
	switch v := item.(type) {
	case []any:
		for idx, x := range v {
			if err := traverseRecursionAndPaths(x, globalDepth+1, funcDepth, fmt.Sprintf("%s/%d", currentPath, idx)); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, pathKey := range []string{"path", "@path"} {
			if p, ok := v[pathKey].(string); ok {
				if !isValidRelaxedPath(p) {
					return newValidationError(
						"VALIDATION_ERROR",
						fmt.Sprintf("Invalid path syntax: '%s'", p),
						currentPath+"/"+pathKey,
						"",
					)
				}
			}
		}
		_, isFCWrapper := v["functionCall"].(map[string]any)
		_, hasCall := v["call"]
		_, hasAtCall := v["@call"]
		_, hasArgs := v["args"]
		isBareFC := (hasCall || hasAtCall) && hasArgs
		if isFCWrapper {
			if funcDepth >= MaxFuncCallDepth {
				return newValidationError(
					"MAX_DEPTH_EXCEEDED",
					fmt.Sprintf("Recursion limit exceeded: functionCall depth > %d", MaxFuncCallDepth),
					currentPath,
					"",
				)
			}
			for _, k := range sortedKeys(v) {
				nextFunc := funcDepth
				if k == "functionCall" {
					nextFunc = funcDepth + 1
				}
				if err := traverseRecursionAndPaths(v[k], globalDepth+1, nextFunc, currentPath+"/"+k); err != nil {
					return err
				}
			}
		} else if isBareFC {
			if funcDepth >= MaxFuncCallDepth {
				return newValidationError(
					"MAX_DEPTH_EXCEEDED",
					fmt.Sprintf("Recursion limit exceeded: functionCall depth > %d", MaxFuncCallDepth),
					currentPath,
					"",
				)
			}
			for _, k := range sortedKeys(v) {
				nextFunc := funcDepth
				if k == "args" {
					nextFunc = funcDepth + 1
				}
				if err := traverseRecursionAndPaths(v[k], globalDepth+1, nextFunc, currentPath+"/"+k); err != nil {
					return err
				}
			}
		} else {
			for _, k := range sortedKeys(v) {
				if err := traverseRecursionAndPaths(v[k], globalDepth+1, funcDepth, currentPath+"/"+k); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func isValidRelaxedPath(p string) bool {
	for i := 0; i < len(p); i++ {
		if p[i] == '~' {
			if i+1 >= len(p) || (p[i+1] != '0' && p[i+1] != '1') {
				return false
			}
			i++
		}
	}
	return true
}

func validateTargetVersion(rawMsg any, msgPath string) error {
	msg, ok := rawMsg.(map[string]any)
	if !ok {
		return nil
	}
	verVal, hasVer := msg["version"]
	if !hasVer {
		return nil
	}
	if verStr, ok := verVal.(string); ok && verStr != "" {
		norm := canonicalVersion(verStr)
		if norm != TargetA2UIVersion {
			return newValidationError(
				"PROTOCOL_VERSION_MISMATCH",
				fmt.Sprintf("Message version '%s' does not match expected target version '%s'", verStr, TargetA2UIVersion),
				msgPath+"/version",
				"",
			)
		}
	}
	return nil
}

func canonicalVersion(v string) string {
	s := strings.TrimSpace(v)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	s = strings.ReplaceAll(s, "_", ".")
	switch s {
	case "1.0", "1.0.0":
		return "1.0"
	case "0.9", "0.9.0":
		return "0.9"
	case "0.9.1":
		return "0.9.1"
	case "0.8", "0.8.0":
		return "0.8"
	default:
		return v
	}
}

type extractedOp struct {
	kind          string
	surfaceID     string
	catalogID     string
	hasCatalogID  bool
	components    []map[string]any
	hasComponents bool
}

func extractMessageOperation(rawMsg any, _ int, msgPath string) (extractedOp, error) {
	msg, ok := rawMsg.(map[string]any)
	if !ok {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: root: Expected object, received %s", jsonTypeOf(rawMsg)),
			msgPath,
			"",
		)
	}

	for _, key := range nativeActionKeys {
		if actionVal, ok := msg[key].(map[string]any); ok {
			if sid, exists := actionVal["surfaceId"]; exists && sid != nil {
				if _, ok := sid.(string); !ok {
					return extractedOp{}, newValidationError(
						"VALIDATION_ERROR",
						"surfaceId must be a string",
						msgPath+"/"+key+"/surfaceId",
						"",
					)
				}
			}
		}
	}

	var present []string
	for _, key := range nativeActionKeys {
		if _, exists := msg[key]; exists {
			present = append(present, key)
		}
	}
	if len(present) > 1 {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Message contains multiple conflicting update actions: %s.", strings.Join(present, ", ")),
			msgPath,
			"",
		)
	}
	if len(present) == 0 {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: message must contain exactly one update action: %s.", strings.Join(nativeActionKeys, ", ")),
			msgPath,
			"",
		)
	}

	verVal, hasVer := msg["version"]
	if !hasVer {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			"Invalid 1.0 message: version: Expected \"1.0\", received undefined",
			msgPath+"/version",
			"",
		)
	}
	if verStr, ok := verVal.(string); !ok || verStr != TargetA2UIVersion {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: version: Invalid literal value, expected %q", TargetA2UIVersion),
			msgPath+"/version",
			"",
		)
	}

	actionKey := present[0]
	if extra := unrecognizedKeys(msg, "version", actionKey); len(extra) > 0 {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: root: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)),
			msgPath,
			"",
		)
	}

	switch actionKey {
	case "createSurface":
		return parseCreateSurfaceOp(msg["createSurface"], msgPath+"/createSurface")
	case "updateComponents":
		return parseUpdateComponentsOp(msg["updateComponents"], msgPath+"/updateComponents")
	case "deleteSurface":
		if err := validateDeleteSurfaceEnvelope(msg["deleteSurface"], msgPath+"/deleteSurface"); err != nil {
			return extractedOp{}, err
		}
		return extractedOp{kind: "deleteSurface"}, nil
	case "updateDataModel":
		if err := validateUpdateDataModelEnvelope(msg["updateDataModel"], msgPath+"/updateDataModel"); err != nil {
			return extractedOp{}, err
		}
		return extractedOp{kind: "updateDataModel"}, nil
	case "callRendererFunction":
		return extractedOp{kind: "callRendererFunction"}, nil
	case "agentFunctionResponse":
		return extractedOp{kind: "agentFunctionResponse"}, nil
	default:
		return extractedOp{kind: actionKey}, nil
	}
}

func parseCreateSurfaceOp(raw any, path string) (extractedOp, error) {
	cs, ok := raw.(map[string]any)
	if !ok {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: createSurface: Expected object, received %s", jsonTypeOf(raw)),
			path,
			"",
		)
	}
	if extra := unrecognizedKeys(cs, "surfaceId", "catalogId", "sendDataModel", "components", "dataModel", "metadata"); len(extra) > 0 {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: createSurface: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)),
			path,
			"",
		)
	}
	sidVal, hasSid := cs["surfaceId"]
	if !hasSid {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			"Invalid 1.0 message: createSurface.surfaceId: Expected string, received undefined",
			path+"/surfaceId",
			"",
		)
	}
	sid, ok := sidVal.(string)
	if !ok || strings.TrimSpace(sid) == "" {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: createSurface.surfaceId: Expected non-empty string, received %s", jsonTypeOf(sidVal)),
			path+"/surfaceId",
			"",
		)
	}

	var catalogID string
	var hasCatalogID bool
	if catVal, exists := cs["catalogId"]; exists {
		catStr, ok := catVal.(string)
		if !ok {
			return extractedOp{}, newValidationError(
				"INVALID_MESSAGE",
				fmt.Sprintf("Invalid 1.0 message: createSurface.catalogId: Expected string, received %s", jsonTypeOf(catVal)),
				path+"/catalogId",
				"",
			)
		}
		catalogID = catStr
		hasCatalogID = true
	}

	if sdmVal, exists := cs["sendDataModel"]; exists {
		if _, ok := sdmVal.(bool); !ok {
			return extractedOp{}, newValidationError(
				"INVALID_MESSAGE",
				fmt.Sprintf("Invalid 1.0 message: createSurface.sendDataModel: Expected boolean, received %s", jsonTypeOf(sdmVal)),
				path+"/sendDataModel",
				"",
			)
		}
	}

	if dmVal, exists := cs["dataModel"]; exists {
		if _, ok := dmVal.(map[string]any); !ok {
			return extractedOp{}, newValidationError(
				"INVALID_MESSAGE",
				fmt.Sprintf("Invalid 1.0 message: createSurface.dataModel: Expected object, received %s", jsonTypeOf(dmVal)),
				path+"/dataModel",
				"",
			)
		}
	}

	if metaVal, exists := cs["metadata"]; exists {
		if err := validateMetadataObject(metaVal, "createSurface.metadata", path+"/metadata", ""); err != nil {
			return extractedOp{}, err
		}
	}

	var comps []map[string]any
	var hasComponents bool
	if compsVal, exists := cs["components"]; exists {
		parsedComps, err := parseComponentsList(compsVal, "createSurface.components", path+"/components")
		if err != nil {
			return extractedOp{}, err
		}
		comps = parsedComps
		hasComponents = true
	}

	return extractedOp{
		kind:          "createSurface",
		surfaceID:     sid,
		catalogID:     catalogID,
		hasCatalogID:  hasCatalogID,
		components:    comps,
		hasComponents: hasComponents,
	}, nil
}

func parseUpdateComponentsOp(raw any, path string) (extractedOp, error) {
	uc, ok := raw.(map[string]any)
	if !ok {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: updateComponents: Expected object, received %s", jsonTypeOf(raw)),
			path,
			"",
		)
	}
	if extra := unrecognizedKeys(uc, "surfaceId", "components"); len(extra) > 0 {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: updateComponents: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)),
			path,
			"",
		)
	}
	sidVal, hasSid := uc["surfaceId"]
	if !hasSid {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			"Invalid 1.0 message: updateComponents.surfaceId: Expected string, received undefined",
			path+"/surfaceId",
			"",
		)
	}
	sid, ok := sidVal.(string)
	if !ok || strings.TrimSpace(sid) == "" {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: updateComponents.surfaceId: Expected non-empty string, received %s", jsonTypeOf(sidVal)),
			path+"/surfaceId",
			"",
		)
	}
	compsVal, hasComps := uc["components"]
	if !hasComps {
		return extractedOp{}, newValidationError(
			"INVALID_MESSAGE",
			"Invalid 1.0 message: updateComponents.components: Expected array, received undefined",
			path+"/components",
			"",
		)
	}
	comps, err := parseComponentsList(compsVal, "updateComponents.components", path+"/components")
	if err != nil {
		return extractedOp{}, err
	}
	return extractedOp{
		kind:          "updateComponents",
		surfaceID:     sid,
		components:    comps,
		hasComponents: true,
	}, nil
}

func validateDeleteSurfaceEnvelope(raw any, path string) error {
	ds, ok := raw.(map[string]any)
	if !ok {
		return newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: deleteSurface: Expected object, received %s", jsonTypeOf(raw)),
			path,
			"",
		)
	}
	if extra := unrecognizedKeys(ds, "surfaceId"); len(extra) > 0 {
		return newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: deleteSurface: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)),
			path,
			"",
		)
	}
	sidVal, hasSid := ds["surfaceId"]
	if !hasSid {
		return newValidationError(
			"INVALID_MESSAGE",
			"Invalid 1.0 message: deleteSurface.surfaceId: Expected string, received undefined",
			path+"/surfaceId",
			"",
		)
	}
	if _, ok := sidVal.(string); !ok {
		return newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: deleteSurface.surfaceId: Expected string, received %s", jsonTypeOf(sidVal)),
			path+"/surfaceId",
			"",
		)
	}
	return nil
}

func validateUpdateDataModelEnvelope(raw any, path string) error {
	ud, ok := raw.(map[string]any)
	if !ok {
		return newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: updateDataModel: Expected object, received %s", jsonTypeOf(raw)),
			path,
			"",
		)
	}
	if extra := unrecognizedKeys(ud, "surfaceId", "path", "value"); len(extra) > 0 {
		return newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: updateDataModel: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)),
			path,
			"",
		)
	}
	sidVal, hasSid := ud["surfaceId"]
	if !hasSid {
		return newValidationError(
			"INVALID_MESSAGE",
			"Invalid 1.0 message: updateDataModel.surfaceId: Expected string, received undefined",
			path+"/surfaceId",
			"",
		)
	}
	if _, ok := sidVal.(string); !ok {
		return newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: updateDataModel.surfaceId: Expected string, received %s", jsonTypeOf(sidVal)),
			path+"/surfaceId",
			"",
		)
	}
	if pathVal, exists := ud["path"]; exists {
		if _, ok := pathVal.(string); !ok {
			return newValidationError(
				"INVALID_MESSAGE",
				fmt.Sprintf("Invalid 1.0 message: updateDataModel.path: Expected string, received %s", jsonTypeOf(pathVal)),
				path+"/path",
				"",
			)
		}
	}
	return nil
}

func parseComponentsList(raw any, label, path string) ([]map[string]any, error) {
	arr, ok := raw.([]any)
	if !ok {
		return nil, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: %s: Expected array, received %s", label, jsonTypeOf(raw)),
			path,
			"",
		)
	}
	if len(arr) == 0 {
		return nil, newValidationError(
			"INVALID_MESSAGE",
			fmt.Sprintf("Invalid 1.0 message: %s: Array must contain at least 1 element(s)", label),
			path,
			"",
		)
	}
	out := make([]map[string]any, 0, len(arr))
	for i, item := range arr {
		comp, ok := item.(map[string]any)
		if !ok {
			return nil, newValidationError(
				"INVALID_MESSAGE",
				fmt.Sprintf("Invalid 1.0 message: %s.%d: Expected object, received %s", label, i, jsonTypeOf(item)),
				fmt.Sprintf("%s/%d", path, i),
				"",
			)
		}
		if compType, ok := comp["component"].(string); ok && compType == "Surface" {
			id, _ := comp["id"].(string)
			return nil, newValidationError(
				"INVALID_MESSAGE",
				fmt.Sprintf("Invalid 1.0 message: %s.%d: Component type cannot be \"Surface\". \"Surface\" is a top-level protocol container defined in createSurface, not a child component.", label, i),
				fmt.Sprintf("%s/%d/component", path, i),
				id,
			)
		}
		out = append(out, comp)
	}
	return out, nil
}

func validateMetadataObject(raw any, label, path, compID string) error {
	meta, ok := raw.(map[string]any)
	if !ok {
		return newValidationError(
			"VALIDATION_ERROR",
			fmt.Sprintf("Invalid 1.0 message: %s: Expected object, received %s", label, jsonTypeOf(raw)),
			path,
			compID,
		)
	}
	if extra := unrecognizedKeys(meta, "extensions"); len(extra) > 0 {
		return newValidationError(
			"VALIDATION_ERROR",
			fmt.Sprintf("Invalid 1.0 message: %s: Unrecognized key(s) in object: %s", label, formatQuotedKeys(extra)),
			path,
			compID,
		)
	}
	if extVal, exists := meta["extensions"]; exists {
		ext, ok := extVal.(map[string]any)
		if !ok {
			return newValidationError(
				"VALIDATION_ERROR",
				fmt.Sprintf("Invalid 1.0 message: %s.extensions: Expected object, received %s", label, jsonTypeOf(extVal)),
				path+"/extensions",
				compID,
			)
		}
		for _, k := range sortedKeys(ext) {
			if !isValidUAX31Identifier(k) {
				return newValidationError(
					"VALIDATION_ERROR",
					fmt.Sprintf("Invalid extension key %q: Keys MUST be Unicode identifiers (UAX #31).", k),
					path+"/extensions/"+k,
					compID,
				)
			}
		}
	}
	return nil
}

type childRef struct {
	id    string
	field string
}

type componentEntry struct {
	id         string
	compType   string
	properties map[string]any
	refs       []childRef
}

type surfaceState struct {
	id             string
	defaultCatalog string
	order          []string
	components     map[string]*componentEntry
}

func applyUpdateComponents(s *surfaceState, batch []map[string]any, allowOrphans bool, basePath string) error {
	seenBatch := make(map[string]bool, len(batch))
	parsedBatch := make([]*componentEntry, 0, len(batch))

	for i, comp := range batch {
		compPath := fmt.Sprintf("%s/components/%d", basePath, i)
		idVal, hasID := comp["id"]
		id, ok := idVal.(string)
		if !hasID || !ok || id == "" {
			compName, _ := comp["component"].(string)
			return newValidationError(
				"INVALID_COMPONENT_ID",
				fmt.Sprintf("Component '%s' is missing an 'id'; entries require a valid string 'id'.", compName),
				compPath+"/id",
				"",
			)
		}
		if seenBatch[id] {
			return newValidationError(
				"DUPLICATE_COMPONENT_ID",
				fmt.Sprintf("Duplicate component ID: '%s'", id),
				compPath+"/id",
				id,
			)
		}
		seenBatch[id] = true

		if !isValidUAX31Identifier(id) {
			return newValidationError(
				"INVALID_COMPONENT_ID",
				fmt.Sprintf("Component id '%s' must be a valid UAX #31 identifier", id),
				compPath+"/id",
				id,
			)
		}

		if catVal, hasCat := comp["catalogId"]; hasCat {
			catStr, ok := catVal.(string)
			if !ok || catStr != CatalogID {
				return newValidationError(
					"CATALOG_ERROR",
					fmt.Sprintf("Component '%s' names unknown catalog '%v'.", id, catVal),
					compPath+"/catalogId",
					id,
				)
			}
		} else if s.defaultCatalog != CatalogID {
			return newValidationError(
				"CATALOG_ERROR",
				fmt.Sprintf("No default catalog for surface '%s', and Component '%s' names no catalogId.", s.id, id),
				compPath,
				id,
			)
		}

		compType, hasType := comp["component"].(string)
		if !hasType || compType == "" {
			if existing, exists := s.components[id]; exists {
				compType = existing.compType
			} else {
				return newValidationError(
					"MISSING_COMPONENT_TYPE",
					fmt.Sprintf("Cannot create component %s without a type.", id),
					compPath+"/component",
					id,
				)
			}
		}

		if !knownCatalogComponents[compType] {
			return newValidationError(
				"UNKNOWN_COMPONENT_TYPE",
				fmt.Sprintf("Component '%s' uses '%s', which is not a component this app can show.", id, compType),
				compPath+"/component",
				id,
			)
		}

		if metaVal, exists := comp["metadata"]; exists {
			if err := validateMetadataObject(metaVal, fmt.Sprintf("component '%s' (%s) metadata", compType, id), compPath+"/metadata", id); err != nil {
				return err
			}
		}

		props := stripComponentEnvelopeKeys(comp)
		if accVal, exists := props["accessibility"]; exists {
			delete(props, "accessibility")
			if err := validateAccessibilityProps(accVal, compType, id, compPath+"/accessibility"); err != nil {
				return err
			}
		}

		refs, err := validateComponentProperties(compType, id, props, compPath)
		if err != nil {
			return err
		}
		parsedBatch = append(parsedBatch, &componentEntry{
			id:         id,
			compType:   compType,
			properties: props,
			refs:       refs,
		})
	}

	candidateOrder := append([]string(nil), s.order...)
	candidateMap := make(map[string]*componentEntry, len(s.components)+len(parsedBatch))
	for k, v := range s.components {
		candidateMap[k] = v
	}
	for _, entry := range parsedBatch {
		if _, exists := candidateMap[entry.id]; !exists {
			candidateOrder = append(candidateOrder, entry.id)
		}
		candidateMap[entry.id] = entry
	}

	if err := validateCompositionConstraints(candidateOrder, candidateMap, basePath); err != nil {
		return err
	}
	if err := validateCandidateTopology(candidateOrder, candidateMap, allowOrphans, basePath); err != nil {
		return err
	}

	s.order = candidateOrder
	s.components = candidateMap
	return nil
}

func stripComponentEnvelopeKeys(comp map[string]any) map[string]any {
	props := make(map[string]any, len(comp))
	for k, v := range comp {
		switch k {
		case "id", "component", "catalogId", "metadata":
			continue
		default:
			props[k] = v
		}
	}
	return props
}

func validateAccessibilityProps(raw any, compType, id, path string) error {
	acc, ok := raw.(map[string]any)
	if !ok {
		return componentValidationFailed(compType, id, fmt.Sprintf("accessibility: Expected object, received %s", jsonTypeOf(raw)), path)
	}
	if extra := unrecognizedKeys(acc, "label", "description", "live", "hidden"); len(extra) > 0 {
		return componentValidationFailed(compType, id, fmt.Sprintf("accessibility: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)), path)
	}
	if labelVal, exists := acc["label"]; exists {
		if !isValidDynamicString(labelVal) {
			return componentValidationFailed(compType, id, fmt.Sprintf("accessibility.label: Expected string, received %s", jsonTypeOf(labelVal)), path+"/label")
		}
	}
	if descVal, exists := acc["description"]; exists {
		if !isValidDynamicString(descVal) {
			return componentValidationFailed(compType, id, fmt.Sprintf("accessibility.description: Expected string, received %s", jsonTypeOf(descVal)), path+"/description")
		}
	}
	if liveVal, exists := acc["live"]; exists {
		if err := checkEnumValue(liveVal, "accessibility.live", []string{"off", "polite", "assertive"}, compType, id, path+"/live"); err != nil {
			return err
		}
	}
	if hiddenVal, exists := acc["hidden"]; exists {
		if _, ok := hiddenVal.(bool); !ok && !isDataBindingOrCall(hiddenVal) {
			return componentValidationFailed(compType, id, fmt.Sprintf("accessibility.hidden: Expected boolean, received %s", jsonTypeOf(hiddenVal)), path+"/hidden")
		}
	}
	return nil
}

func validateComponentProperties(compType, id string, props map[string]any, compPath string) ([]childRef, error) {
	switch compType {
	case "Column":
		return validateColumnOrRowProps(compType, id, props, compPath, []string{"start", "center", "end", "spaceBetween", "spaceAround", "spaceEvenly", "stretch"}, []string{"center", "end", "start", "stretch"})
	case "Row":
		return validateColumnOrRowProps(compType, id, props, compPath, []string{"center", "end", "spaceAround", "spaceBetween", "spaceEvenly", "start", "stretch"}, []string{"start", "center", "end", "stretch"})
	case "Tabs":
		return validateTabsProps(compType, id, props, compPath)
	case "Card":
		return validateCardProps(compType, id, props, compPath)
	case "Text":
		return nil, validateTextProps(compType, id, props, compPath)
	case "KeyPoints":
		return nil, validateKeyPointsProps(compType, id, props, compPath)
	case "MetricGrid":
		return nil, validateMetricGridProps(compType, id, props, compPath)
	case "ComparisonTable":
		return nil, validateComparisonTableProps(compType, id, props, compPath)
	case "Timeline":
		return nil, validateTimelineProps(compType, id, props, compPath)
	case "Checklist":
		return nil, validateChecklistProps(compType, id, props, compPath)
	default:
		return nil, newValidationError(
			"UNKNOWN_COMPONENT_TYPE",
			fmt.Sprintf("Component '%s' uses '%s', which is not a component this app can show.", id, compType),
			compPath+"/component",
			id,
		)
	}
}

func validateColumnOrRowProps(compType, id string, props map[string]any, compPath string, justifyOpts, alignOpts []string) ([]childRef, error) {
	if extra := unrecognizedKeys(props, "children", "justify", "align", "weight"); len(extra) > 0 {
		return nil, componentValidationFailed(compType, id, fmt.Sprintf("root: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)), compPath)
	}
	childrenVal, hasChildren := props["children"]
	if !hasChildren {
		return nil, componentValidationFailed(compType, id, "children: Expected array, received undefined", compPath+"/children")
	}
	refs, err := parseChildList(childrenVal, compType, id, compPath+"/children")
	if err != nil {
		return nil, err
	}
	if justifyVal, exists := props["justify"]; exists {
		if err := checkEnumValue(justifyVal, "justify", justifyOpts, compType, id, compPath+"/justify"); err != nil {
			return nil, err
		}
	}
	if alignVal, exists := props["align"]; exists {
		if err := checkEnumValue(alignVal, "align", alignOpts, compType, id, compPath+"/align"); err != nil {
			return nil, err
		}
	}
	if weightVal, exists := props["weight"]; exists {
		if _, ok := weightVal.(float64); !ok {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("weight: Expected number, received %s", jsonTypeOf(weightVal)), compPath+"/weight")
		}
	}
	return refs, nil
}

func parseChildList(raw any, compType, id, path string) ([]childRef, error) {
	switch v := raw.(type) {
	case []any:
		refs := make([]childRef, 0, len(v))
		for i, item := range v {
			childID, ok := item.(string)
			if !ok {
				return nil, componentValidationFailed(compType, id, fmt.Sprintf("children.%d: Expected string, received %s", i, jsonTypeOf(item)), fmt.Sprintf("%s/%d", path, i))
			}
			refs = append(refs, childRef{id: childID, field: fmt.Sprintf("children[%d]", i)})
		}
		return refs, nil
	case map[string]any:
		if extra := unrecognizedKeys(v, "componentId", "path"); len(extra) > 0 {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("children: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)), path)
		}
		cidVal, hasCid := v["componentId"]
		if !hasCid {
			return nil, componentValidationFailed(compType, id, "children.componentId: Expected string, received undefined", path+"/componentId")
		}
		cid, ok := cidVal.(string)
		if !ok {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("children.componentId: Expected string, received %s", jsonTypeOf(cidVal)), path+"/componentId")
		}
		pVal, hasPath := v["path"]
		if !hasPath {
			return nil, componentValidationFailed(compType, id, "children.path: Expected string, received undefined", path+"/path")
		}
		if _, ok := pVal.(string); !ok {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("children.path: Expected string, received %s", jsonTypeOf(pVal)), path+"/path")
		}
		return []childRef{{id: cid, field: "children.componentId"}}, nil
	default:
		return nil, componentValidationFailed(compType, id, fmt.Sprintf("children: Expected array, received %s", jsonTypeOf(raw)), path)
	}
}

func validateTabsProps(compType, id string, props map[string]any, compPath string) ([]childRef, error) {
	if extra := unrecognizedKeys(props, "tabs", "weight"); len(extra) > 0 {
		return nil, componentValidationFailed(compType, id, fmt.Sprintf("root: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)), compPath)
	}
	tabsVal, hasTabs := props["tabs"]
	if !hasTabs {
		return nil, componentValidationFailed(compType, id, "tabs: Expected array, received undefined", compPath+"/tabs")
	}
	tabsArr, ok := tabsVal.([]any)
	if !ok {
		return nil, componentValidationFailed(compType, id, fmt.Sprintf("tabs: Expected array, received %s", jsonTypeOf(tabsVal)), compPath+"/tabs")
	}
	if len(tabsArr) < 1 {
		return nil, componentValidationFailed(compType, id, "tabs: Array must contain at least 1 element(s)", compPath+"/tabs")
	}
	refs := make([]childRef, 0, len(tabsArr))
	for i, rawTab := range tabsArr {
		tabPath := fmt.Sprintf("%s/tabs/%d", compPath, i)
		tab, ok := rawTab.(map[string]any)
		if !ok {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("tabs.%d: Expected object, received %s", i, jsonTypeOf(rawTab)), tabPath)
		}
		if extra := unrecognizedKeys(tab, "title", "child"); len(extra) > 0 {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("tabs.%d: Unrecognized key(s) in object: %s", i, formatQuotedKeys(extra)), tabPath)
		}
		titleVal, hasTitle := tab["title"]
		if !hasTitle {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("tabs.%d.title: Expected string, received undefined", i), tabPath+"/title")
		}
		if !isValidDynamicString(titleVal) {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("tabs.%d.title: Expected string, received %s", i, jsonTypeOf(titleVal)), tabPath+"/title")
		}
		childVal, hasChild := tab["child"]
		if !hasChild {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("tabs.%d.child: Expected string, received undefined", i), tabPath+"/child")
		}
		childID, ok := childVal.(string)
		if !ok {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("tabs.%d.child: Expected string, received %s", i, jsonTypeOf(childVal)), tabPath+"/child")
		}
		refs = append(refs, childRef{id: childID, field: fmt.Sprintf("tabs[%d].child", i)})
	}
	if weightVal, exists := props["weight"]; exists {
		if _, ok := weightVal.(float64); !ok {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("weight: Expected number, received %s", jsonTypeOf(weightVal)), compPath+"/weight")
		}
	}
	return refs, nil
}

func validateCardProps(compType, id string, props map[string]any, compPath string) ([]childRef, error) {
	if extra := unrecognizedKeys(props, "child", "weight"); len(extra) > 0 {
		return nil, componentValidationFailed(compType, id, fmt.Sprintf("root: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)), compPath)
	}
	childVal, hasChild := props["child"]
	if !hasChild {
		return nil, componentValidationFailed(compType, id, "child: Expected string, received undefined", compPath+"/child")
	}
	childID, ok := childVal.(string)
	if !ok {
		return nil, componentValidationFailed(compType, id, fmt.Sprintf("child: Expected string, received %s", jsonTypeOf(childVal)), compPath+"/child")
	}
	if weightVal, exists := props["weight"]; exists {
		if _, ok := weightVal.(float64); !ok {
			return nil, componentValidationFailed(compType, id, fmt.Sprintf("weight: Expected number, received %s", jsonTypeOf(weightVal)), compPath+"/weight")
		}
	}
	return []childRef{{id: childID, field: "child"}}, nil
}

func validateTextProps(compType, id string, props map[string]any, compPath string) error {
	if extra := unrecognizedKeys(props, "text", "variant", "weight"); len(extra) > 0 {
		return componentValidationFailed(compType, id, fmt.Sprintf("root: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)), compPath)
	}
	textVal, hasText := props["text"]
	if !hasText {
		return componentValidationFailed(compType, id, "text: Expected string, received undefined", compPath+"/text")
	}
	if !isValidDynamicString(textVal) {
		return componentValidationFailed(compType, id, fmt.Sprintf("text: Expected string, received %s", jsonTypeOf(textVal)), compPath+"/text")
	}
	if variantVal, exists := props["variant"]; exists {
		if err := checkEnumValue(variantVal, "variant", []string{"caption", "body"}, compType, id, compPath+"/variant"); err != nil {
			return err
		}
	}
	if weightVal, exists := props["weight"]; exists {
		if _, ok := weightVal.(float64); !ok {
			return componentValidationFailed(compType, id, fmt.Sprintf("weight: Expected number, received %s", jsonTypeOf(weightVal)), compPath+"/weight")
		}
	}
	return nil
}

func validateContentCommonProps(compType, id string, props map[string]any, compPath string) error {
	if titleVal, exists := props["title"]; exists {
		if _, ok := titleVal.(string); !ok {
			return componentValidationFailed(compType, id, fmt.Sprintf("title: Expected string, received %s", jsonTypeOf(titleVal)), compPath+"/title")
		}
	}
	if sourcesVal, exists := props["sources"]; exists {
		arr, ok := sourcesVal.([]any)
		if !ok {
			return componentValidationFailed(compType, id, fmt.Sprintf("sources: Expected array, received %s", jsonTypeOf(sourcesVal)), compPath+"/sources")
		}
		for i, item := range arr {
			if _, ok := item.(string); !ok {
				return componentValidationFailed(compType, id, fmt.Sprintf("sources.%d: Expected string, received %s", i, jsonTypeOf(item)), fmt.Sprintf("%s/sources/%d", compPath, i))
			}
		}
	}
	if weightVal, exists := props["weight"]; exists {
		w, ok := weightVal.(float64)
		if !ok {
			return componentValidationFailed(compType, id, fmt.Sprintf("weight: Expected number, received %s", jsonTypeOf(weightVal)), compPath+"/weight")
		}
		if w <= 0 {
			return componentValidationFailed(compType, id, "weight: Number must be greater than 0", compPath+"/weight")
		}
	}
	return nil
}

func validateKeyPointsProps(compType, id string, props map[string]any, compPath string) error {
	if extra := unrecognizedKeys(props, "title", "items", "sources", "weight"); len(extra) > 0 {
		return componentValidationFailed(compType, id, fmt.Sprintf("root: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)), compPath)
	}
	if err := validateContentCommonProps(compType, id, props, compPath); err != nil {
		return err
	}
	itemsVal, hasItems := props["items"]
	if !hasItems {
		return componentValidationFailed(compType, id, "items: Expected array, received undefined", compPath+"/items")
	}
	items, ok := itemsVal.([]any)
	if !ok {
		return componentValidationFailed(compType, id, fmt.Sprintf("items: Expected array, received %s", jsonTypeOf(itemsVal)), compPath+"/items")
	}
	if len(items) < 1 {
		return componentValidationFailed(compType, id, "items: Array must contain at least 1 element(s)", compPath+"/items")
	}
	for i, item := range items {
		if _, ok := item.(string); !ok {
			return componentValidationFailed(compType, id, fmt.Sprintf("items.%d: Expected string, received %s", i, jsonTypeOf(item)), fmt.Sprintf("%s/items/%d", compPath, i))
		}
	}
	return nil
}

func validateMetricGridProps(compType, id string, props map[string]any, compPath string) error {
	if extra := unrecognizedKeys(props, "title", "metrics", "sources", "weight"); len(extra) > 0 {
		return componentValidationFailed(compType, id, fmt.Sprintf("root: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)), compPath)
	}
	if err := validateContentCommonProps(compType, id, props, compPath); err != nil {
		return err
	}
	metricsVal, hasMetrics := props["metrics"]
	if !hasMetrics {
		return componentValidationFailed(compType, id, "metrics: Expected array, received undefined", compPath+"/metrics")
	}
	metrics, ok := metricsVal.([]any)
	if !ok {
		return componentValidationFailed(compType, id, fmt.Sprintf("metrics: Expected array, received %s", jsonTypeOf(metricsVal)), compPath+"/metrics")
	}
	if len(metrics) < 1 {
		return componentValidationFailed(compType, id, "metrics: Array must contain at least 1 element(s)", compPath+"/metrics")
	}
	for i, rawMetric := range metrics {
		metricPath := fmt.Sprintf("%s/metrics/%d", compPath, i)
		m, ok := rawMetric.(map[string]any)
		if !ok {
			return componentValidationFailed(compType, id, fmt.Sprintf("metrics.%d: Expected object, received %s", i, jsonTypeOf(rawMetric)), metricPath)
		}
		if extra := unrecognizedKeys(m, "label", "value", "unit", "change", "trend"); len(extra) > 0 {
			return componentValidationFailed(compType, id, fmt.Sprintf("metrics.%d: Unrecognized key(s) in object: %s", i, formatQuotedKeys(extra)), metricPath)
		}
		if err := checkRequiredString(m, "label", fmt.Sprintf("metrics.%d.label", i), compType, id, metricPath+"/label"); err != nil {
			return err
		}
		if err := checkRequiredString(m, "value", fmt.Sprintf("metrics.%d.value", i), compType, id, metricPath+"/value"); err != nil {
			return err
		}
		if unitVal, exists := m["unit"]; exists {
			if _, ok := unitVal.(string); !ok {
				return componentValidationFailed(compType, id, fmt.Sprintf("metrics.%d.unit: Expected string, received %s", i, jsonTypeOf(unitVal)), metricPath+"/unit")
			}
		}
		if changeVal, exists := m["change"]; exists {
			if _, ok := changeVal.(string); !ok {
				return componentValidationFailed(compType, id, fmt.Sprintf("metrics.%d.change: Expected string, received %s", i, jsonTypeOf(changeVal)), metricPath+"/change")
			}
		}
		if trendVal, exists := m["trend"]; exists {
			if err := checkEnumValue(trendVal, fmt.Sprintf("metrics.%d.trend", i), []string{"up", "down", "flat"}, compType, id, metricPath+"/trend"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateComparisonTableProps(compType, id string, props map[string]any, compPath string) error {
	if extra := unrecognizedKeys(props, "title", "columns", "rows", "sources", "weight"); len(extra) > 0 {
		return componentValidationFailed(compType, id, fmt.Sprintf("root: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)), compPath)
	}
	if err := validateContentCommonProps(compType, id, props, compPath); err != nil {
		return err
	}
	colsVal, hasCols := props["columns"]
	if !hasCols {
		return componentValidationFailed(compType, id, "columns: Expected array, received undefined", compPath+"/columns")
	}
	cols, ok := colsVal.([]any)
	if !ok {
		return componentValidationFailed(compType, id, fmt.Sprintf("columns: Expected array, received %s", jsonTypeOf(colsVal)), compPath+"/columns")
	}
	if len(cols) < 2 {
		return componentValidationFailed(compType, id, "columns: Array must contain at least 2 element(s)", compPath+"/columns")
	}
	for i, c := range cols {
		if _, ok := c.(string); !ok {
			return componentValidationFailed(compType, id, fmt.Sprintf("columns.%d: Expected string, received %s", i, jsonTypeOf(c)), fmt.Sprintf("%s/columns/%d", compPath, i))
		}
	}

	rowsVal, hasRows := props["rows"]
	if !hasRows {
		return componentValidationFailed(compType, id, "rows: Expected array, received undefined", compPath+"/rows")
	}
	rows, ok := rowsVal.([]any)
	if !ok {
		return componentValidationFailed(compType, id, fmt.Sprintf("rows: Expected array, received %s", jsonTypeOf(rowsVal)), compPath+"/rows")
	}
	if len(rows) < 1 {
		return componentValidationFailed(compType, id, "rows: Array must contain at least 1 element(s)", compPath+"/rows")
	}
	for i, rawRow := range rows {
		rowPath := fmt.Sprintf("%s/rows/%d", compPath, i)
		row, ok := rawRow.([]any)
		if !ok {
			return componentValidationFailed(compType, id, fmt.Sprintf("rows.%d: Expected array, received %s", i, jsonTypeOf(rawRow)), rowPath)
		}
		for j, cell := range row {
			if _, ok := cell.(string); !ok {
				return componentValidationFailed(compType, id, fmt.Sprintf("rows.%d.%d: Expected string, received %s", i, j, jsonTypeOf(cell)), fmt.Sprintf("%s/%d", rowPath, j))
			}
		}
		if len(row) != len(cols) {
			return componentValidationFailed(
				compType,
				id,
				fmt.Sprintf("rows.%d: Row %d has %d cells; there are %d columns.", i, i+1, len(row), len(cols)),
				rowPath,
			)
		}
	}
	return nil
}

func validateTimelineProps(compType, id string, props map[string]any, compPath string) error {
	if extra := unrecognizedKeys(props, "title", "events", "sources", "weight"); len(extra) > 0 {
		return componentValidationFailed(compType, id, fmt.Sprintf("root: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)), compPath)
	}
	if err := validateContentCommonProps(compType, id, props, compPath); err != nil {
		return err
	}
	eventsVal, hasEvents := props["events"]
	if !hasEvents {
		return componentValidationFailed(compType, id, "events: Expected array, received undefined", compPath+"/events")
	}
	events, ok := eventsVal.([]any)
	if !ok {
		return componentValidationFailed(compType, id, fmt.Sprintf("events: Expected array, received %s", jsonTypeOf(eventsVal)), compPath+"/events")
	}
	if len(events) < 1 {
		return componentValidationFailed(compType, id, "events: Array must contain at least 1 element(s)", compPath+"/events")
	}
	for i, rawEvent := range events {
		eventPath := fmt.Sprintf("%s/events/%d", compPath, i)
		ev, ok := rawEvent.(map[string]any)
		if !ok {
			return componentValidationFailed(compType, id, fmt.Sprintf("events.%d: Expected object, received %s", i, jsonTypeOf(rawEvent)), eventPath)
		}
		if extra := unrecognizedKeys(ev, "when", "title", "detail"); len(extra) > 0 {
			return componentValidationFailed(compType, id, fmt.Sprintf("events.%d: Unrecognized key(s) in object: %s", i, formatQuotedKeys(extra)), eventPath)
		}
		if err := checkRequiredString(ev, "when", fmt.Sprintf("events.%d.when", i), compType, id, eventPath+"/when"); err != nil {
			return err
		}
		if err := checkRequiredString(ev, "title", fmt.Sprintf("events.%d.title", i), compType, id, eventPath+"/title"); err != nil {
			return err
		}
		if detailVal, exists := ev["detail"]; exists {
			if _, ok := detailVal.(string); !ok {
				return componentValidationFailed(compType, id, fmt.Sprintf("events.%d.detail: Expected string, received %s", i, jsonTypeOf(detailVal)), eventPath+"/detail")
			}
		}
	}
	return nil
}

func validateChecklistProps(compType, id string, props map[string]any, compPath string) error {
	if extra := unrecognizedKeys(props, "title", "items", "sources", "weight"); len(extra) > 0 {
		return componentValidationFailed(compType, id, fmt.Sprintf("root: Unrecognized key(s) in object: %s", formatQuotedKeys(extra)), compPath)
	}
	if err := validateContentCommonProps(compType, id, props, compPath); err != nil {
		return err
	}
	itemsVal, hasItems := props["items"]
	if !hasItems {
		return componentValidationFailed(compType, id, "items: Expected array, received undefined", compPath+"/items")
	}
	items, ok := itemsVal.([]any)
	if !ok {
		return componentValidationFailed(compType, id, fmt.Sprintf("items: Expected array, received %s", jsonTypeOf(itemsVal)), compPath+"/items")
	}
	if len(items) < 1 {
		return componentValidationFailed(compType, id, "items: Array must contain at least 1 element(s)", compPath+"/items")
	}
	for i, rawItem := range items {
		itemPath := fmt.Sprintf("%s/items/%d", compPath, i)
		it, ok := rawItem.(map[string]any)
		if !ok {
			return componentValidationFailed(compType, id, fmt.Sprintf("items.%d: Expected object, received %s", i, jsonTypeOf(rawItem)), itemPath)
		}
		if extra := unrecognizedKeys(it, "text", "done", "quote"); len(extra) > 0 {
			return componentValidationFailed(compType, id, fmt.Sprintf("items.%d: Unrecognized key(s) in object: %s", i, formatQuotedKeys(extra)), itemPath)
		}
		if err := checkRequiredString(it, "text", fmt.Sprintf("items.%d.text", i), compType, id, itemPath+"/text"); err != nil {
			return err
		}
		if doneVal, exists := it["done"]; exists {
			if _, ok := doneVal.(bool); !ok {
				return componentValidationFailed(compType, id, fmt.Sprintf("items.%d.done: Expected boolean, received %s", i, jsonTypeOf(doneVal)), itemPath+"/done")
			}
		}
		if quoteVal, exists := it["quote"]; exists {
			if _, ok := quoteVal.(string); !ok {
				return componentValidationFailed(compType, id, fmt.Sprintf("items.%d.quote: Expected string, received %s", i, jsonTypeOf(quoteVal)), itemPath+"/quote")
			}
		}
	}
	return nil
}

func validateCompositionConstraints(order []string, components map[string]*componentEntry, basePath string) error {
	type parentInfo struct {
		parentID   string
		parentType string
	}
	parentsOf := make(map[string][]parentInfo)
	for _, parentID := range order {
		entry := components[parentID]
		for _, ref := range entry.refs {
			parentsOf[ref.id] = append(parentsOf[ref.id], parentInfo{
				parentID:   parentID,
				parentType: entry.compType,
			})
		}
	}

	for _, id := range order {
		entry := components[id]
		var allowedParents []string
		switch entry.compType {
		case "Row":
			allowedParents = rowAllowedParents
		case "Tabs":
			allowedParents = tabsAllowedParents
		}
		if len(allowedParents) > 0 {
			if id == "root" && !sliceContains(allowedParents, "Surface") {
				allowedJSON, _ := json.Marshal(allowedParents)
				msg := fmt.Sprintf("UNALLOWED_PARENT: Component '%s' (%s) cannot be placed under parent 'Surface' (Surface). Allowed parents: %s.", id, entry.compType, allowedJSON)
				return newValidationError("UNALLOWED_PARENT", msg, basePath+"/components", id)
			}
			for _, p := range parentsOf[id] {
				if !sliceContains(allowedParents, p.parentType) {
					allowedJSON, _ := json.Marshal(allowedParents)
					msg := fmt.Sprintf("UNALLOWED_PARENT: Component '%s' (%s) cannot be placed under parent '%s' (%s). Allowed parents: %s.", id, entry.compType, p.parentID, p.parentType, allowedJSON)
					return newValidationError("UNALLOWED_PARENT", msg, basePath+"/components", id)
				}
			}
		}

		if entry.compType == "Card" {
			for _, ref := range entry.refs {
				childEntry, exists := components[ref.id]
				if !exists {
					continue
				}
				if !sliceContains(cardAllowedChildren, childEntry.compType) {
					allowedJSON, _ := json.Marshal(cardAllowedChildren)
					msg := fmt.Sprintf("UNALLOWED_CHILD: Container '%s' (%s) cannot contain child '%s' (%s). Allowed children: %s.", id, entry.compType, ref.id, childEntry.compType, allowedJSON)
					return newValidationError("UNALLOWED_CHILD", msg, basePath+"/components", id)
				}
			}
		}
	}
	return nil
}

func validateCandidateTopology(order []string, components map[string]*componentEntry, allowOrphans bool, basePath string) error {
	if len(components) == 0 {
		return nil
	}
	if _, hasRoot := components["root"]; !hasRoot {
		return newValidationError(
			"MISSING_ROOT",
			"Missing root component: No component has id='root'",
			basePath+"/components",
			"root",
		)
	}

	for _, id := range order {
		entry := components[id]
		for _, ref := range entry.refs {
			if _, exists := components[ref.id]; !exists {
				return newValidationError(
					"DANGLING_REFERENCE",
					fmt.Sprintf("Component '%s' references non-existent component '%s' in field '%s' (Dangling reference '%s' in component '%s')", id, ref.id, ref.field, ref.id, id),
					basePath+"/components",
					id,
				)
			}
		}
	}

	for _, id := range order {
		entry := components[id]
		for _, ref := range entry.refs {
			if ref.id == id {
				return newValidationError(
					"CIRCULAR_REFERENCE",
					fmt.Sprintf("Circular reference detected: Component '%s' references itself in field '%s' (Self-reference detected)", id, ref.field),
					basePath+"/components",
					id,
				)
			}
		}
	}

	visited := make(map[string]bool, len(components))
	stack := make(map[string]bool, len(components))

	var dfs func(nodeID string, depth int) error
	dfs = func(nodeID string, depth int) error {
		if depth > MaxGlobalDepth {
			return newValidationError(
				"MAX_DEPTH_EXCEEDED",
				fmt.Sprintf("Global recursion limit exceeded: logical depth > %d", MaxGlobalDepth),
				basePath+"/components",
				nodeID,
			)
		}
		visited[nodeID] = true
		stack[nodeID] = true
		entry := components[nodeID]
		for _, ref := range entry.refs {
			if _, exists := components[ref.id]; !exists {
				continue
			}
			if !visited[ref.id] {
				if err := dfs(ref.id, depth+1); err != nil {
					return err
				}
			} else if stack[ref.id] {
				return newValidationError(
					"CIRCULAR_REFERENCE",
					fmt.Sprintf("Circular reference detected involving component '%s'", ref.id),
					basePath+"/components",
					ref.id,
				)
			}
		}
		delete(stack, nodeID)
		return nil
	}

	if err := dfs("root", 0); err != nil {
		return err
	}

	if !allowOrphans && len(visited) < len(components) {
		var orphans []string
		for _, id := range order {
			if !visited[id] {
				orphans = append(orphans, id)
			}
		}
		sort.Strings(orphans)
		if len(orphans) > 0 {
			first := orphans[0]
			return newValidationError(
				"ORPHAN_COMPONENT",
				fmt.Sprintf("Component '%s' is not reachable from 'root' (Orphaned component '%s' is not reachable from root)", first, first),
				basePath+"/components",
				first,
			)
		}
	}

	return nil
}

func componentValidationFailed(compType, id, detail, path string) *ValidationError {
	return newValidationError(
		"VALIDATION_ERROR",
		fmt.Sprintf("Validation failed for component '%s' (%s): %s", compType, id, detail),
		path,
		id,
	)
}

func checkRequiredString(m map[string]any, key, fieldPath, compType, id, jsonPath string) error {
	val, exists := m[key]
	if !exists {
		return componentValidationFailed(compType, id, fmt.Sprintf("%s: Expected string, received undefined", fieldPath), jsonPath)
	}
	if _, ok := val.(string); !ok {
		return componentValidationFailed(compType, id, fmt.Sprintf("%s: Expected string, received %s", fieldPath, jsonTypeOf(val)), jsonPath)
	}
	return nil
}

func checkEnumValue(val any, fieldPath string, options []string, compType, id, jsonPath string) error {
	str, ok := val.(string)
	if !ok {
		return componentValidationFailed(
			compType,
			id,
			fmt.Sprintf("%s: Expected %s, received %s", fieldPath, strings.Join(options, " | "), jsonTypeOf(val)),
			jsonPath,
		)
	}
	if !sliceContains(options, str) {
		return componentValidationFailed(
			compType,
			id,
			fmt.Sprintf("%s: Invalid enum value. Expected %s, received '%s'", fieldPath, strings.Join(options, " | "), str),
			jsonPath,
		)
	}
	return nil
}

func isValidDynamicString(val any) bool {
	if _, ok := val.(string); ok {
		return true
	}
	return isDataBindingOrCall(val)
}

func isDataBindingOrCall(val any) bool {
	obj, ok := val.(map[string]any)
	if !ok {
		return false
	}
	if p, ok := obj["@path"].(string); ok && isValidRelaxedPath(p) && len(unrecognizedKeys(obj, "@path", "path")) == 0 {
		return true
	}
	if p, ok := obj["path"].(string); ok && isValidRelaxedPath(p) && len(unrecognizedKeys(obj, "@path", "path")) == 0 {
		return true
	}
	if c, ok := obj["@call"].(string); ok && c != "" {
		return true
	}
	if c, ok := obj["call"].(string); ok && c != "" {
		return true
	}
	return false
}

func isValidUAX31Identifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) && !unicode.In(r, unicode.Nl) {
				return false
			}
		} else {
			if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.In(r, unicode.Nl, unicode.Mn, unicode.Mc, unicode.Pc) {
				return false
			}
		}
	}
	return true
}

func jsonTypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func unrecognizedKeys(m map[string]any, allowed ...string) []string {
	allowedSet := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = true
	}
	var extra []string
	for k := range m {
		if !allowedSet[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return extra
}

func formatQuotedKeys(keys []string) string {
	quoted := make([]string, len(keys))
	for i, k := range keys {
		quoted[i] = fmt.Sprintf("'%s'", k)
	}
	return strings.Join(quoted, ", ")
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sliceContains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
