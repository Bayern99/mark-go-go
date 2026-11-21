package genui

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/genuikit"
	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

const (
	statementFaultFloor = 2
	statementFaultShare = 10
)

var statementName = regexp.MustCompile(`^\s*(\$?[A-Za-z_][A-Za-z0-9_]*)\s*=`)

type statementMessage struct {
	CreateSurface    *statementBatch `json:"createSurface"`
	UpdateComponents *statementBatch `json:"updateComponents"`
}

type statementBatch struct {
	SurfaceID  string   `json:"surfaceId"`
	CatalogID  string   `json:"catalogId"`
	Components []string `json:"components"`
}

func WritesStatements(a2ui json.RawMessage) bool {
	var messages []statementMessage
	return json.Unmarshal(a2ui, &messages) == nil && len(messages) > 0 && messages[0].CreateSurface != nil && messages[0].CreateSurface.CatalogID == StatementCatalogID
}

func eachSourceID(value any, visit func(string)) {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			eachSourceID(item, visit)
		}
	case *openuilang.Object:
		for _, key := range v.Keys {
			eachSourceID(v.Values[key], visit)
		}
	case *openuilang.Element:
		if v == nil {
			return
		}
		if v.Type == "Source" {
			ids, _ := v.Props.Values["sources"].([]any)
			for _, id := range ids {
				if text, ok := id.(string); ok {
					visit(text)
				}
			}
		}
		eachSourceID(v.Props, visit)
	}
}

func validateStatementCandidate(raw []byte) error {
	view, err := ReadStatementView(raw)
	if err != nil {
		return err
	}
	library, err := genuikit.Library()
	if err != nil {
		return err
	}
	text, count := "", 0
	for _, batch := range view.Statements {
		text = openuilang.Overlay(text, strings.Join(batch, "\n"))
		count += len(batch)
	}
	program := library.Parse(text)
	session := openuilang.NewSession()
	session.Load(program)
	root, problems := session.Render()
	if root == nil {
		return newValidationError("NO_ROOT", "The view has no root statement, such as root = Col([...]).", "/a2ui/0/createSurface/components", "")
	}
	seen := map[string]bool{}
	var faults []string
	for _, problem := range append(program.Errors, problems...) {
		fault := problem.Message
		if problem.StatementID != "" {
			fault = problem.StatementID + ": " + fault
		}
		if !seen[fault] {
			seen[fault] = true
			faults = append(faults, fault)
		}
	}
	for _, name := range program.Unresolved {
		faults = append(faults, fmt.Sprintf("%q is used but never defined", name))
	}
	if allowed := max(statementFaultFloor, count/statementFaultShare); len(faults) > allowed {
		return newValidationError("TOO_MANY_FAULTS", fmt.Sprintf("The view has %d faulty statements, more than the %d it may have. The first are: %s.", len(faults), allowed, strings.Join(faults[:min(3, len(faults))], "; ")), "/a2ui", "")
	}
	var missing error
	eachSourceID(root, func(id string) {
		if _, ok := view.Sources[id]; !ok && missing == nil {
			missing = newValidationError("MISSING_SOURCE", fmt.Sprintf("A Source names %q, which is not in the view's sources.", id), "/sources", "")
		}
	})
	return missing
}

func readStatementBase(text string, ids []string) (*BaseView, error) {
	var file struct {
		Sources json.RawMessage   `json:"sources"`
		A2UI    []json.RawMessage `json:"a2ui"`
	}
	if _, err := ReadStatementView([]byte(text)); err != nil {
		return nil, notUpdatable("it is not a valid version 3 view file.")
	}
	_ = json.Unmarshal([]byte(text), &file)
	sources, err := ParseSources(file.Sources)
	if err != nil {
		return nil, notUpdatable("its sources are not valid.")
	}
	base := &BaseView{raw: []byte(text), sources: sources, messages: file.A2UI, existing: map[string]bool{}, replace: map[string]bool{}, statements: true}
	for _, raw := range file.A2UI {
		var message statementMessage
		_ = json.Unmarshal(raw, &message)
		batch := message.UpdateComponents
		if message.CreateSurface != nil {
			batch = message.CreateSurface
			base.surfaceID = batch.SurfaceID
		}
		for _, statement := range batch.Components {
			if match := statementName.FindStringSubmatch(statement); match != nil {
				base.existing[match[1]] = true
			}
		}
	}
	for _, id := range ids {
		if !base.existing[id] {
			return nil, notUpdatable(fmt.Sprintf("it has no statement %q.", id))
		}
		base.replace[id] = true
	}
	return base, nil
}

func (b *BaseView) checkStatements(update json.RawMessage) error {
	var batch struct {
		SurfaceID  string `json:"surfaceId"`
		Components []any  `json:"components"`
	}
	if err := json.Unmarshal(update, &batch); err != nil || len(batch.Components) == 0 {
		return errors.New(`The agent's message is not an updateComponents with components.`)
	}
	if batch.SurfaceID != b.surfaceID {
		return fmt.Errorf("The agent's update is for the surface %q; the view's surface is %q.", batch.SurfaceID, b.surfaceID)
	}
	seen := map[string]bool{}
	for _, component := range batch.Components {
		statement, ok := component.(string)
		match := statementName.FindStringSubmatch(statement)
		switch {
		case !ok || match == nil:
			return errors.New("The agent's update has a component that is not a statement, such as name = Text(\"...\").")
		case seen[match[1]]:
			return fmt.Errorf("The agent's update names the statement %q twice.", match[1])
		case b.existing[match[1]] && !b.replace[match[1]]:
			return fmt.Errorf("The agent's update changes the statement %q, which it was not asked to replace.", match[1])
		}
		seen[match[1]] = true
	}
	return nil
}

func statementPrompt(b *strings.Builder, base *BaseView) {
	if base == nil {
		b.WriteString("Make a complete view for the reader's request, using all relevant sections; when no request is supplied, represent the document's substance. out.json is a JSON object with \"sources\", the source references by id, and \"a2ui\", a list holding exactly one A2UI 1.0 createSurface message for the surface \"view\" with the catalog \"" + StatementCatalogID + "\". Its \"components\" is a list of strings, each one openui-lang statement, and the tree starts at the statement \"root\". Use only the components and functions in the format definition. Leave out version, language, generatedAt, model and the sources' hash: the app writes them.\n\n")
		return
	}
	fmt.Fprintf(b, "The document changed since its view was made. Make an update for the current view below. out.json is a JSON object with \"a2ui\", a list holding exactly one A2UI 1.0 updateComponents message for the surface %q with only the keys \"version\" and \"updateComponents\", and \"sources\", the source references it uses. Its \"components\" is a list of strings, each one openui-lang statement. Replace only these statements, whose sources changed: %s. Keep the names of the statements you replace. ", base.surfaceID, strings.Join(QuoteAll(base.ReplaceIDs()), ", "))
	b.WriteString("The message may hold only those statements and new statements they refer to, with names the current view does not use yet; every other statement of the view stays exactly as it is, and the replaced statements may still refer to it. ")
	b.WriteString("Sources may reuse the current view's sources with exactly their existing heading and occurrence, or add new ids. Leave out version, language, generatedAt, model and the sources' hash: the app writes them.\n\n")
}
