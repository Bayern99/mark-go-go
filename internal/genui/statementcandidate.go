package genui

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/genuikit"
	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

const (
	statementFaultFloor = 2
	statementFaultShare = 10
)

type viewMessage[Component any] struct {
	CreateSurface    *componentBatch[Component] `json:"createSurface"`
	UpdateComponents *componentBatch[Component] `json:"updateComponents"`
}

type componentBatch[Component any] struct {
	SurfaceID  string      `json:"surfaceId"`
	CatalogID  string      `json:"catalogId"`
	Components []Component `json:"components"`
}

func (m viewMessage[Component]) batch() *componentBatch[Component] {
	if m.CreateSurface != nil {
		return m.CreateSurface
	}
	return m.UpdateComponents
}

type statementMessage = viewMessage[string]

func WritesStatements(a2ui json.RawMessage) bool {
	var messages []statementMessage
	return json.Unmarshal(a2ui, &messages) == nil && len(messages) > 0 && messages[0].CreateSurface != nil && messages[0].CreateSurface.CatalogID == StatementCatalogID
}

func eachSourceID(value any, seen map[*openuilang.Element]bool, visit func(string)) {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			eachSourceID(item, seen, visit)
		}
	case *openuilang.Object:
		for _, key := range v.Keys {
			eachSourceID(v.Values[key], seen, visit)
		}
	case *openuilang.Element:
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		if v.Type == "Source" {
			ids, _ := v.Props.Values["sources"].([]any)
			for _, id := range ids {
				if text, ok := id.(string); ok {
					visit(text)
				}
			}
		}
		eachSourceID(v.Props, seen, visit)
	}
}

type DegradedProperty struct {
	Statement string `json:"statement"`
	Component string `json:"component"`
	Property  string `json:"property"`
	Message   string `json:"message"`
}

type CandidateReport struct {
	Degraded []DegradedProperty
}

func validateStatementCandidate(raw []byte) (CandidateReport, error) {
	var report CandidateReport
	view, err := ReadStatementView(raw)
	if err != nil {
		return report, err
	}
	library, err := genuikit.Library()
	if err != nil {
		return report, err
	}
	text := ""
	for _, batch := range view.Statements {
		text = openuilang.Overlay(text, strings.Join(batch, "\n"))
	}
	count := len(openuilang.Names(text))
	program := library.Parse(text)
	session := openuilang.NewSession()
	session.Load(program)
	root, problems := session.Render()
	if root == nil {
		return report, newValidationError("NO_ROOT", "The view has no root statement, such as root = Col([...]).", "/a2ui/0/createSurface/components", "")
	}
	seen := map[string]bool{}
	var faults []string
	for _, problem := range append(program.Errors, problems...) {
		if problem.Degraded {
			property := DegradedProperty{Statement: problem.StatementID, Component: problem.Component, Property: strings.TrimPrefix(problem.Path, "/"), Message: problem.Message}
			if !slices.Contains(report.Degraded, property) {
				report.Degraded = append(report.Degraded, property)
			}
			continue
		}
		key, fault := problem.StatementID, problem.StatementID+": "+problem.Message
		if key == "" {
			key, fault = problem.Message, problem.Message
		}
		if !seen[key] {
			seen[key] = true
			faults = append(faults, fault)
		}
	}
	for _, name := range program.Unresolved {
		faults = append(faults, fmt.Sprintf("%q is used but never defined", name))
	}
	if allowed := max(statementFaultFloor, count/statementFaultShare); len(faults) > allowed {
		return report, newValidationError("TOO_MANY_FAULTS", fmt.Sprintf("The view has %d faulty statements, more than the %d it may have. The first are: %s.", len(faults), allowed, strings.Join(faults[:min(3, len(faults))], "; ")), "/a2ui", "")
	}
	var missing error
	eachSourceID(root, map[*openuilang.Element]bool{}, func(id string) {
		if _, ok := view.Sources[id]; !ok && missing == nil {
			missing = newValidationError("MISSING_SOURCE", fmt.Sprintf("A Source names %q, which is not in the view's sources.", id), "/sources", "")
		}
	})
	return report, missing
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
		if message.CreateSurface != nil {
			base.surfaceID = message.CreateSurface.SurfaceID
		}
	}
	for _, name := range openuilang.Names(base.statementText()) {
		base.existing[name] = true
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
		names := openuilang.Names(statement)
		if !ok || len(names) != 1 {
			return errors.New("The agent's update has a component that is not one statement, such as name = Text(\"...\").")
		}
		name := names[0]
		switch {
		case seen[name]:
			return fmt.Errorf("The agent's update names the statement %q twice.", name)
		case b.existing[name] && !b.replace[name]:
			return fmt.Errorf("The agent's update changes the statement %q, which it was not asked to replace.", name)
		}
		seen[name] = true
	}
	return nil
}

func (b *BaseView) statementText() string {
	text := ""
	for _, raw := range b.messages {
		var message statementMessage
		_ = json.Unmarshal(raw, &message)
		if batch := message.batch(); batch != nil {
			text = openuilang.Overlay(text, strings.Join(batch.Components, "\n"))
		}
	}
	return text
}

type statementUpdate struct {
	SurfaceID  string   `json:"surfaceId"`
	Components []string `json:"components"`
}

func (b *BaseView) deletingUnreferenced(message json.RawMessage, envelope map[string]json.RawMessage) json.RawMessage {
	var update statementUpdate
	_ = json.Unmarshal(envelope["updateComponents"], &update)
	earlier, written := b.statementText(), strings.Join(update.Components, "\n")
	referenced := map[string]bool{}
	for _, name := range openuilang.Names(openuilang.Merge(earlier, written)) {
		referenced[name] = true
	}
	var deletions []string
	for _, name := range openuilang.Names(openuilang.Overlay(earlier, written)) {
		if !referenced[name] {
			deletions = append(deletions, name+" = null")
		}
	}
	if len(deletions) == 0 {
		return message
	}
	update.Components = append(update.Components, deletions...)
	deleting, _ := json.Marshal(struct {
		Version          json.RawMessage `json:"version,omitempty"`
		UpdateComponents statementUpdate `json:"updateComponents"`
	}{envelope["version"], update})
	return deleting
}

func statementPrompt(b *strings.Builder, base *BaseView) {
	b.WriteString(`Write one strict JSON object, without Markdown fences. A component is a JSON string, so the quotes inside the OpenUI statement must be escaped in the file. Use " for embedded quotes, for example "root = Col([Text("Title")])". Raw quotes inside a component string make the file invalid JSON.` + "\n\n")
	if base == nil {
		b.WriteString("Make a complete view for the reader's request, using all relevant sections; when no request is supplied, represent the document's substance. out.json is a JSON object with \"sources\", the source references by id, and \"a2ui\", a list holding exactly one A2UI 1.0 createSurface message for the surface \"view\" with the catalog \"" + StatementCatalogID + "\". Its \"components\" is a list of strings, each one openui-lang statement, and the tree starts at the statement \"root\". Use only the components and functions in the format definition. Leave out the top-level version, language, generatedAt, model and the sources' hash: the app writes them. Every message in a2ui must include \"version\": \"1.0\" beside its operation.\n")
		fmt.Fprintf(b, "New view example: %s\n\n", `{"sources":{},"a2ui":[{"version":"1.0","createSurface":{"surfaceId":"view","catalogId":"`+StatementCatalogID+`","components":["root = Col([Text(\"Title\")])"]}}]}`)
		return
	}
	fmt.Fprintf(b, "The document changed since its view was made. Make an update for the current view below. out.json is a JSON object with \"a2ui\", a list holding exactly one A2UI 1.0 updateComponents message for the surface %q with only the keys \"version\" and \"updateComponents\", and \"sources\", the source references it uses. Its \"components\" is a list of strings, each one openui-lang statement. Replace only these statements, whose sources changed: %s. Keep the names of the statements you replace. ", base.surfaceID, strings.Join(QuoteAll(base.ReplaceIDs()), ", "))
	b.WriteString("The message may hold only those statements and new statements they refer to, with names the current view does not use yet; every other statement of the view stays exactly as it is, and the replaced statements may still refer to it. ")
	b.WriteString("Sources may reuse the current view's sources with exactly their existing heading and occurrence, or add new ids. Leave out the top-level version, language, generatedAt, model and the sources' hash: the app writes them. Every message in a2ui must include \"version\": \"1.0\" beside its operation.\n")
	fmt.Fprintf(b, "Update message shape: %s\n\n", `{"version":"1.0","updateComponents":{"surfaceId":`+strconv.Quote(base.surfaceID)+`,"components":[...]}}`)
}
