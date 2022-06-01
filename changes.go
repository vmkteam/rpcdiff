package main

import (
	"encoding/json"
	"fmt"
	"github.com/thoas/go-funk"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	openrpc "github.com/vmkteam/meta-schema/v2"
)

type CriticalityLevel string

const (
	Breaking    CriticalityLevel = "BREAKING"
	NonBreaking CriticalityLevel = "NON_BREAKING"
	Dangerous   CriticalityLevel = "DANGEROUS"
)

func (c CriticalityLevel) String() string {
	switch c {
	case Breaking:
		return "breaking"
	case Dangerous:
		return "dangerous"
	case NonBreaking:
		return "non breaking"
	}

	return ""
}

type ChangeType string

const (
	Added   ChangeType = "ADDED"
	Removed ChangeType = "REMOVED"
	Changed ChangeType = "CHANGED"
)

type ChangeObject string

const (
	OpenRPCVersion ChangeObject = "OPEN_RPC_VERSION"

	SchemaInfo    ChangeObject = "SCHEMA_INFO"
	SchemaServers ChangeObject = "SCHEMA_SERVERS"

	Method               ChangeObject = "METHOD"
	MethodParamStructure ChangeObject = "METHOD_PARAM_STRUCTURE"

	MethodParam     ChangeObject = "METHOD_PARAM"
	MethodParamType ChangeObject = "METHOD_PARAM_TYPE" // type + ref + items type + items ref

	MethodResult     ChangeObject = "METHOD_RESULT"
	MethodResultType ChangeObject = "METHOD_RESULT_TYPE" // schema type + ref

	MethodError ChangeObject = "METHOD_ERROR"

	ComponentsSchema             ChangeObject = "COMPONENTS_SCHEMA"
	ComponentsSchemaType         ChangeObject = "COMPONENTS_SCHEMA_TYPE"
	ComponentsSchemaProperty     ChangeObject = "COMPONENTS_SCHEMA_PROPERTY"
	ComponentsSchemaPropertyType ChangeObject = "COMPONENTS_SCHEMA_PROPERTY_TYPE" // type + ref + items type + items ref

	ComponentsDescriptor     ChangeObject = "COMPONENTS_DESCRIPTOR"
	ComponentsDescriptorType ChangeObject = "COMPONENTS_DESCRIPTOR_TYPE"

	Other ChangeObject = "OTHER"
)

type Change struct {
	Path        []string         `json:"path"`
	Type        ChangeType       `json:"type"`
	Object      ChangeObject     `json:"object"`
	Criticality CriticalityLevel `json:"criticality"`
	Old         interface{}
	New         interface{}

	// TODO Add related paths to definitions/schemas diffs
	//Related     []string `json:"related"`
}

func (c *Change) String() string {
	methodName := after(c.Path, "methods")
	paramName := after(c.Path, "params")
	schemaName := after(c.Path, "schemas")
	propName := after(c.Path, "properties")
	descrName := after(c.Path, "contentDescriptors")

	switch c.Object {
	// method
	case Method:
		switch c.Type {
		case Added:
			return fmt.Sprintf(`Added method "%s"`, methodName)
		case Removed:
			return fmt.Sprintf(`Removed method "%s"`, methodName)
		case Changed:
			return fmt.Sprintf(`Changed "%s" at method "%s" from "%v" to "%v"`, last(c.Path), methodName, c.Old, c.New)
		}
	// method param structure
	case MethodParamStructure:
		return fmt.Sprintf(`Changed "%s" at method "%s" from "%v" to "%v"`, last(c.Path), methodName, c.Old, c.New)
	// method param
	case MethodParam:
		if contains(c.Path, "required") {
			return fmt.Sprintf(`Set as %s arg "%s" at method "%s"`, requiredString(c.Type, c.Old, c.New), paramName, methodName)
		}

		switch c.Type {
		case Added:
			req := "optional"
			if c.Criticality == Breaking {
				req = "required"
			}

			return fmt.Sprintf(`Added %s arg "%s" to method "%s"`, req, last(c.Path), methodName)
		case Removed:
			return fmt.Sprintf(`Removed arg "%s" from method "%s"`, last(c.Path), methodName)
		case Changed:
			return fmt.Sprintf(`Changed "%s" at method "%s(%s)" from "%v" to "%v"`, last(c.Path), methodName, paramName, c.Old, c.New)
		}
	case MethodParamType:
		return fmt.Sprintf(`Changed type of arg "%s" at method "%s" from "%v" to "%v"`, paramName, methodName, c.Old, c.New)
	case MethodResult:
		if last(c.Path) == "result" {
			return fmt.Sprintf(`Changed result of method "%s" from "%v" to "%v"`, methodName, c.Old, c.New)
		}
		return fmt.Sprintf(`Changed "%s" at result of method "%s" from "%v" to "%v"`, last(c.Path), methodName, c.Old, c.New)
	case MethodResultType:
		return fmt.Sprintf(`Changed result type of method "%s" from "%v" to "%v"`, methodName, c.Old, c.New)
	case MethodError:
		switch c.Type {
		case Added:
			return fmt.Sprintf(`Added error "%s" to method "%s"`, last(c.Path), methodName)
		case Removed:
			return fmt.Sprintf(`Removed error "%s" from method "%s"`, last(c.Path), methodName)
		case Changed:
			return fmt.Sprintf(`Changed "%s" at error "%s" of method "%s" from "%v" to "%v"`, last(c.Path), after(c.Path, "errors"), methodName, c.Old, c.New)
		}
	case ComponentsSchema:
		if contains(c.Path, "required") {
			pName := c.New
			if isNil(c.New) {
				pName = c.Old
			}

			return fmt.Sprintf(`Set as %s param "%v" at schema "%s"`, requiredString(c.Type, c.Old, c.New), pName, schemaName)
		}

		switch c.Type {
		case Added:
			return fmt.Sprintf(`Added schema "%s"`, schemaName)
		case Removed:
			return fmt.Sprintf(`Removed schema "%s"`, schemaName)
		case Changed:
			return fmt.Sprintf(`Changed "%s" at schema "%s" from "%v" to "%v"`, last(c.Path), schemaName, c.Old, c.New)
		}
	case ComponentsSchemaType:
		return fmt.Sprintf(`Changed type of schema "%s" from "%v" to "%v"`, schemaName, c.Old, c.New)
	case ComponentsSchemaProperty:
		switch c.Type {
		case Added:
			return fmt.Sprintf(`Added prop "%s" to schema "%s"`, last(c.Path), schemaName)
		case Removed:
			return fmt.Sprintf(`Removed prop "%s" from schema "%s"`, last(c.Path), schemaName)
		case Changed:
			return fmt.Sprintf(`Changed "%s" at schema "%s(%s)" from "%v" to "%v"`, last(c.Path), schemaName, propName, c.Old, c.New)
		}
	case ComponentsSchemaPropertyType:
		return fmt.Sprintf(`Changed type of prop "%s" of schema "%s" from "%v" to "%v"`, propName, schemaName, c.Old, c.New)
	case ComponentsDescriptor:
		switch c.Type {
		case Added:
			return fmt.Sprintf(`Added descriptor "%s"`, descrName)
		case Removed:
			return fmt.Sprintf(`Removed descriptor "%s"`, descrName)
		case Changed:
			return fmt.Sprintf(`Changed "%s" at descriptor "%s" from "%v" to "%v"`, last(c.Path), descrName, c.Old, c.New)
		}
	case ComponentsDescriptorType:
		return fmt.Sprintf(`Changed type of descriptor "%s" from "%v" to "%v"`, descrName, c.Old, c.New)
	default:
		switch c.Type {
		case Added:
			return fmt.Sprintf(`Added %s "%s"`, last(c.Path), c.Path[0])
		case Removed:
			return fmt.Sprintf(`Removed %s from "%s"`, last(c.Path), c.Path[0])
		case Changed:
			return fmt.Sprintf(`Changed "%s" at "%s" from "%v" to "%v"`, last(c.Path), c.Path[0], c.Old, c.New)
		}
	}

	return ""
}

func requiredString(typ ChangeType, from, to interface{}) string {
	switch typ {
	case Added:
		return "required"
	case Removed:
		return "not required"
	}

	if isTrue(to) {
		return "required"
	}
	if isTrue(from) {
		return "not required"
	}

	return ""
}

type Diff struct {
	Criticality CriticalityLevel `json:"criticality"`
	Changes     []Change         `json:"changes"`
	Options     Options          `json:"-"`
}

type Options struct {
	ShowMeta bool
}

func NewDiff(old, new string, options Options) (*Diff, error) {
	oldBytes, err := readFileOrUrl(old)
	if err != nil {
		return nil, fmt.Errorf("read old schema error: %w", err)
	}

	newBytes, err := readFileOrUrl(new)
	if err != nil {
		return nil, fmt.Errorf("read new schema error: %w", err)
	}

	return NewDiffBytes(oldBytes, newBytes, options)
}

func readFileOrUrl(path string) ([]byte, error) {
	if _, err := url.ParseRequestURI(path); err != nil {
		return ioutil.ReadFile(path)
	}

	resp, err := http.Get(path)
	if err != nil {
		return nil, err
	}

	if resp.Body != nil {
		defer resp.Body.Close()
		return ioutil.ReadAll(resp.Body)
	}

	return []byte{}, nil
}

func NewDiffBytes(oldJSON, newJSON []byte, options Options) (*Diff, error) {
	var oldSchema openrpc.OpenrpcDocument
	if err := json.Unmarshal(oldJSON, &oldSchema); err != nil {
		return nil, err
	}

	var newSchema openrpc.OpenrpcDocument
	if err := json.Unmarshal(newJSON, &newSchema); err != nil {
		return nil, err
	}

	diff := &Diff{
		Criticality: NonBreaking,
		Options:     options,
	}

	diff.Changes = compareDocument(options, &oldSchema, &newSchema)

	for _, c := range diff.Changes {
		if c.Criticality == Dangerous {
			diff.Criticality = Dangerous
		}
		if c.Criticality == Breaking {
			diff.Criticality = Breaking
			break
		}
	}

	return diff, nil
}

func (d *Diff) String() string {
	if len(d.Changes) == 0 {
		return "There is no difference between schemas"
	}

	buf := strings.Builder{}
	fmt.Fprintf(&buf, "New schema has %s change(s)\n", d.Criticality.String())

	changesMap := map[CriticalityLevel][]Change{
		Breaking:    {},
		Dangerous:   {},
		NonBreaking: {},
	}

	for _, change := range d.Changes {
		changesMap[change.Criticality] = append(changesMap[change.Criticality], change)
	}

	for _, level := range []CriticalityLevel{Breaking, Dangerous, NonBreaking} {
		if len(changesMap[level]) > 0 {
			fmt.Fprintf(&buf, "%s changes (%d):\n", strings.Title(level.String()), len(changesMap[level]))
			for _, change := range changesMap[level] {
				fmt.Fprintf(&buf, "- %s\n", change.String())
			}
		}
	}

	return buf.String()
}

// compareDocument compares two openrpc documents recursively
func compareDocument(options Options, old, new *openrpc.OpenrpcDocument) []Change {
	var changes []Change

	// openrpc version
	if change := compare(old.Openrpc, new.Openrpc, []string{"openrpc"}, Breaking); change != nil {
		changes = append(changes, *change)
	}

	// info object
	changes = append(changes, compareInfo(options, old.Info, new.Info)...)

	// servers object
	changes = append(changes, compareServers(options, old.Servers, new.Servers)...)

	// methods
	changes = append(changes, compareMethods(options, old.Methods, new.Methods)...)

	// components
	changes = append(changes, compareComponents(options, old, new)...)

	return changes
}

// compareInfo compares info sections recursively
func compareInfo(options Options, old, new *openrpc.InfoObject) []Change {
	if !options.ShowMeta {
		return nil
	}

	// basic compare
	return compareRecursive(old, new, []string{"info"}, nil)
}

// compareServers compares servers sections recursively
func compareServers(options Options, old, new []openrpc.ServerObject) []Change {
	if !options.ShowMeta {
		return nil
	}

	// basic compare
	return compareRecursive(old, new, []string{"servers"}, nil)
}

// compareMethods compares each method with counterpart recursively
func compareMethods(options Options, old, new []openrpc.MethodOrReference) []Change {
	var changes []Change

	oldMap := map[string]openrpc.MethodOrReference{}
	for _, method := range old {
		oldMap[method.Name] = method
	}

	newMap := map[string]openrpc.MethodOrReference{}
	for _, method := range new {
		newMap[method.Name] = method
	}

	for oldMethodName, oldMethod := range oldMap {
		if newMethod, ok := newMap[oldMethodName]; ok {
			changes = append(changes, compareMethod(options, oldMethod, newMethod, []string{"methods", oldMethodName})...)

			delete(newMap, oldMethodName)
		} else {
			// breaking on method delete
			changes = append(changes, *compare(oldMethod, nil, []string{"methods", oldMethodName}, Breaking))
		}
	}

	for newMethodName, newMethod := range newMap {
		// non-breaking on method add
		changes = append(changes, *compare(nil, newMethod, []string{"methods", newMethodName}, NonBreaking))
	}

	return changes
}

// compareMethod compares two methods recursively
func compareMethod(options Options, old, new openrpc.MethodOrReference, path []string) []Change {
	var changes []Change

	// param structure
	if old.ParamStructure != new.ParamStructure {
		changes = append(changes, compareParamStructure(old.ParamStructure, new.ParamStructure, append(path, "paramStructure")))
	}

	// params thyself
	changes = append(changes, compareMethodParams(options, old.Params, new.Params, append(path, "params"))...)

	// results
	changes = append(changes, compareMethodResults(options, old.Result, new.Result, append(path, "result"))...)

	// errors
	changes = append(changes, compareMethodErrors(options, old.Errors, new.Errors, append(path, "errors"))...)

	// rest of the fields
	changes = append(changes, compareRecursive(old, new, path, []string{"paramStructure", "params", "result", "errors"})...)

	return changes
}

// compareParamStructure compares two methods' param structure recursively
func compareParamStructure(old, new openrpc.MethodObjectParamStructure, path []string) Change {
	var criticality CriticalityLevel

	for {
		if new == openrpc.MethodObjectParamStructureEnum2 || new == "" {
			criticality = NonBreaking
			break
		}

		if old == openrpc.MethodObjectParamStructureEnum2 || old == "" {
			criticality = Dangerous
			break
		}

		if old != new {
			criticality = Breaking
			break
		}
		break
	}

	return Change{
		Path:        path,
		Type:        detectChangeType(old, new),
		Object:      MethodParamStructure,
		Criticality: criticality,
		Old:         old,
		New:         new,
	}
}

// compareMethodParams compares params of two methods
func compareMethodParams(options Options, old, new []openrpc.ContentDescriptorOrReference, path []string) []Change {
	var changes []Change

	oldMap := map[string]openrpc.ContentDescriptorOrReference{}
	for _, param := range old {
		oldMap[param.Name] = param
	}

	newMap := map[string]openrpc.ContentDescriptorOrReference{}
	for _, param := range new {
		newMap[param.Name] = param
	}

	for oldParamName, oldParam := range oldMap {
		if newParam, ok := newMap[oldParamName]; ok {
			changes = append(changes, compareContentDescriptor(options, oldParam, newParam, append(path, oldParamName), true)...)

			delete(newMap, oldParamName)
		} else {
			// non-breaking on param delete
			changes = append(changes, *compare(oldParam, nil, append(path, oldParamName), NonBreaking))
		}
	}

	for newParamName, newParam := range newMap {
		level := NonBreaking
		if newParam.Required {
			level = Breaking
		}

		// non-breaking on method add
		changes = append(changes, *compare(nil, newParam, append(path, newParamName), level))
	}

	return changes
}

// compareType compares type in JSON Schema
func compareType(options Options, old, new *openrpc.Type, path []string) *Change {
	if reflect.DeepEqual(old, new) {
		return nil
	}

	if old == nil || new == nil {
		return compare(old, new, path, Breaking)
	}

	level := Breaking
	if (old.SimpleType == "integer" || old.SimpleType == "int") && (new.SimpleType == "number" || new.SimpleType == "float") {
		level = NonBreaking
	}

	return compare(old.SimpleType, new.SimpleType, path, level)
}

// compareMethodResults compares results of methods
func compareMethodResults(options Options, old, new *openrpc.MethodObjectResult, path []string) []Change {
	if reflect.DeepEqual(old, new) {
		return nil
	}

	if old == nil {
		return []Change{*compare(old, new, path, NonBreaking)}
	}
	if new == nil {
		return []Change{*compare(old, new, path, Breaking)}
	}

	oldCD := openrpc.ContentDescriptorOrReference{
		ContentDescriptorObject: old.ContentDescriptorObject,
		ReferenceObject:         old.ReferenceObject,
	}

	newCD := openrpc.ContentDescriptorOrReference{
		ContentDescriptorObject: new.ContentDescriptorObject,
		ReferenceObject:         new.ReferenceObject,
	}

	return compareContentDescriptor(options, oldCD, newCD, append(path, "result"), false)
}

// compareMethodErrors compares errors of methods
func compareMethodErrors(options Options, old, new []openrpc.ErrorOrReference, path []string) []Change {
	return compareRecursive(old, new, path, []string{})
}

// compareComponents compares each component
func compareComponents(options Options, oldDoc, newDoc *openrpc.OpenrpcDocument) []Change {
	var changes []Change

	changes = append(changes, compareComponentsSchemas(options, oldDoc.Components.Schemas, newDoc.Components.Schemas, oldDoc, newDoc)...)

	return changes
}

// compareComponents compares each component
func compareComponentsSchemas(options Options, old, new *openrpc.SchemaMap, oldDoc, newDoc *openrpc.OpenrpcDocument) []Change {
	var changes []Change

	path := []string{"components", "schemas"}

	if (old != nil) != (new != nil) {
		return []Change{*compare(old, new, path, NonBreaking)}
	}

	if old == nil {
		old = &openrpc.SchemaMap{}
	}

	if new == nil {
		new = &openrpc.SchemaMap{}
	}

	index := map[string]bool{}
	for _, oldSchema := range *old {
		if newSchema, ok := new.Get(oldSchema.Id); ok {
			isInput := detectRequiredInput(newSchema.Id, newDoc, []string{}, 0)

			changes = append(changes, compareJSONSchema(options, oldSchema, newSchema, append(path, oldSchema.Id), isInput)...)

			index[newSchema.Id] = true
		} else {
			changes = append(changes, *compare(oldSchema, nil, append(path, oldSchema.Id), NonBreaking))
		}
	}

	for _, newSchema := range *new {
		if index[newSchema.Id] {
			continue
		}

		changes = append(changes, *compare(nil, newSchema, append(path, newSchema.Id), NonBreaking))
	}

	return changes
}

func detectRequiredInput(name string, doc *openrpc.OpenrpcDocument, checked []string, level int) bool {
	if level > 5 {
		return false
	}

	ref := fmt.Sprintf("#/components/schemas/%s", name)
	// for every method
	for _, method := range doc.Methods {
		// every param
		for _, param := range method.Params {
			// check reference
			paramRef := ""
			if param.ReferenceObject != nil {
				paramRef = param.ReferenceObject.Ref
			} else if param.ContentDescriptorObject != nil && param.Schema != nil {
				paramRef = param.Schema.Ref
			}

			if ref == paramRef && (level == 0 && param.Required || level > 0) {
				return true
			}
		}
	}

	if doc.Components != nil && doc.Components.Schemas != nil {
		for _, schema := range *doc.Components.Schemas {
			if schema.Properties == nil {
				continue
			}

			if funk.ContainsString(checked, schema.Id) {
				continue
			}

			for _, prop := range *schema.Properties {
				if prop.Ref == ref && funk.ContainsString(schema.Required, prop.Title) {
					if detectRequiredInput(schema.Id, doc, append(checked, name), level+1) {
						return true
					}
				}
			}
		}
	}

	return false
}

// compareContentDescriptor compares any kind of content descriptor
func compareContentDescriptor(options Options, old, new openrpc.ContentDescriptorOrReference, path []string, isInput bool) []Change {
	var changes []Change

	// descriptor -> reference
	if old.ContentDescriptorObject != nil && new.ReferenceObject != nil {
		return []Change{*compare(old.ContentDescriptorObject, new.ReferenceObject, path, Breaking)}
	}

	// reference -> descriptor
	if old.ReferenceObject != nil && new.ContentDescriptorObject != nil {
		return []Change{*compare(old.ReferenceObject, new.ContentDescriptorObject, path, Breaking)}
	}

	if change := compareRef(options, old.ReferenceObject, new.ReferenceObject, append(path, "$ref")); change != nil {
		return []Change{*change}
	}

	// required
	if old.Required != new.Required {
		level := NonBreaking
		if new.Required {
			level = Breaking
		}

		changes = append(changes, *compare(old.Required, new.Required, append(path, "required"), level))
	}

	// schema
	changes = append(changes, compareJSONSchema(options, *old.Schema, *new.Schema, append(path, "schema"), isInput)...)

	// summary
	if old.Summary != new.Summary {
		changes = append(changes, *compare(old.Summary, new.Summary, append(path, "summary"), NonBreaking))
	}

	// description
	if old.Description != new.Description {
		changes = append(changes, *compare(old.Description, new.Description, append(path, "description"), NonBreaking))
	}

	return changes
}

// compareJSONSchema compares any kind of json schema
func compareJSONSchema(options Options, old, new openrpc.JSONSchema, path []string, isInput bool) []Change {
	var changes []Change

	if reflect.DeepEqual(old, new) {
		return nil
	}

	// reference -> schema or schema -> reference
	if (old.Ref != "") != (new.Ref != "") {
		return []Change{*compare(old, new, path, Breaking)}
	}

	if change := compareRef(options, old.Ref, new.Ref, append(path, "$ref")); change != nil {
		return []Change{*change}
	}

	// type
	if change := compareType(options, old.Type, new.Type, append(path, "type")); change != nil {
		changes = append(changes, *change)
	}

	// items
	if !reflect.DeepEqual(old.Items, new.Items) {
		if old.Items != nil && new.Items != nil {
			changes = append(changes, compareJSONSchema(options, *old.Items.JSONSchema, *new.Items.JSONSchema, append(path, "items"), isInput)...)
		} else if old.Items == nil {
			changes = append(changes, *compare(nil, new.Items, append(path, "items"), Breaking))
		} else if new.Items == nil {
			changes = append(changes, *compare(new.Items, nil, append(path, "items"), Breaking))
		}
	}

	// required
	if c := compareRecursive(old.Required, new.Required, append(path, "required"), []string{}); len(c) > 0 {
		for i := range c {
			if c[i].Type == Added && isInput {
				c[i].Criticality = Breaking
			}
		}

		changes = append(changes, c...)
	}

	// properties
	changes = append(changes, compareJSONSchemaProperties(options, old.Properties, new.Properties, append(path, "properties"), isInput)...)

	// rest of the fields
	changes = append(changes, compareRecursive(old, new, path, []string{"required", "items", "type", "$ref", "properties"})...)

	return changes
}

// compareJSONSchemaProperties compares properties of json schemas
func compareJSONSchemaProperties(options Options, old, new *openrpc.SchemaMap, path []string, isInput bool) []Change {
	var changes []Change

	if reflect.DeepEqual(old, new) {
		return nil
	}

	if old == nil {
		return []Change{*compare(nil, new, path, NonBreaking)}
	}

	if new == nil {
		return []Change{*compare(old, nil, path, NonBreaking)}
	}

	index := map[string]bool{}
	for _, oldSchema := range *old {
		if newSchema, ok := new.Get(oldSchema.Id); ok {
			changes = append(changes, compareJSONSchema(options, oldSchema, newSchema, append(path, oldSchema.Id), isInput)...)

			index[newSchema.Id] = true
		} else {
			// non-breaking on schema delete
			changes = append(changes, *compare(oldSchema, nil, append(path, oldSchema.Id), Dangerous))
		}
	}

	for _, newSchema := range *new {
		if index[newSchema.Id] {
			continue
		}
		// non-breaking on method add
		changes = append(changes, *compare(nil, newSchema, append(path, newSchema.Id), NonBreaking))
	}

	return changes
}

// compareRef compares reference objects or reference strings
func compareRef(options Options, old, new interface{}, path []string) *Change {
	if reflect.DeepEqual(old, new) {
		return nil
	}

	if !sameType(old, new) || isNil(old) || isNil(new) {
		return compare(old, new, path, Breaking)
	}

	var oldVal, newVal string

	switch ov := old.(type) {
	case string:
		newVal = new.(string)
		oldVal = ov
	case openrpc.ReferenceObject:
		newVal = new.(openrpc.ReferenceObject).Ref
		oldVal = ov.Ref
	case *openrpc.ReferenceObject:
		if nv := new.(*openrpc.ReferenceObject); nv != nil {
			newVal = nv.Ref
		}
		if ov != nil {
			oldVal = ov.Ref
		}
	}

	if oldVal == "" {
		return compare(nil, newVal, path, Breaking)
	}
	if newVal == "" {
		return compare(oldVal, nil, path, Breaking)
	}

	return compare(oldVal, newVal, path, Breaking)
}

// compareRecursive is generic compare function for any type
func compareRecursive(old, new interface{}, p, exclude []string) []Change {
	path := make([]string, len(p))
	copy(path, p)

	var changes []Change
	if funk.ContainsString(exclude, last(path)) {
		return nil
	}

	if isNil(old) != isNil(new) {
		return []Change{*compare(old, new, path, NonBreaking)}
	}

	if !sameType(old, new) {
		changes = append(changes, *compare(old, new, path, NonBreaking))
	}

	// embed simple types
	if okOld, oldE := getEmbedSimpleType(old); okOld {
		if okNew, newE := getEmbedSimpleType(new); okNew {
			if change := compare(oldE, newE, path, NonBreaking); change != nil {
				changes = append(changes, *change)
			}

			return changes
		}
	}

	if (isStruct(old) && isStruct(new)) || (isMap(old) && isMap(new)) || (isSlice(old) && isSlice(new)) {
		oldMap := getMap(old)
		newMap := getMap(new)

		for oldFieldName, oldFieldVal := range oldMap {
			if newFieldVal, ok := newMap[oldFieldName]; ok {
				changes = append(changes, compareRecursive(oldFieldVal, newFieldVal, append(path, oldFieldName), exclude)...)

				delete(newMap, oldFieldName)
			} else {
				changes = append(changes, compareRecursive(oldFieldVal, nil, append(path, oldFieldName), exclude)...)
			}
		}

		for leftFieldName, leftFieldVal := range newMap {
			changes = append(changes, compareRecursive(nil, leftFieldVal, append(path, leftFieldName), exclude)...)
		}

		return changes
	}

	if change := compare(old, new, path, NonBreaking); change != nil {
		changes = append(changes, *change)
	}

	return changes
}

func getEmbedSimpleType(v interface{}) (bool, interface{}) {
	if isNil(v) {
		return false, v
	}

	if isStruct(v) {
		st := structs.New(v)
		for _, field := range st.Fields() {
			if field.IsEmbedded() && field.IsExported() {
				value := field.Value()
				if !isStruct(value) && !isMap(value) && !isSlice(value) && !field.IsZero() {
					return true, value
				}
			}
		}
	}

	return false, v
}

func getMap(v interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	if isNil(v) {
		return result
	}

	s := reflect.ValueOf(v)
	if s.Kind() == reflect.Ptr || s.Kind() == reflect.Interface {
		s = s.Elem()
	}

	switch s.Kind() {
	case reflect.Slice:
		for i := 0; i < s.Len(); i++ {
			val := s.Index(i).Interface()

			if isStruct(val) {
				if key, val := getIdentifier(val); key != nil {
					result[*key] = val
					continue
				}
			}

			result[fmt.Sprintf("%d", i)] = val
		}
	case reflect.Map:
		for _, k := range s.MapKeys() {
			result[fmt.Sprintf("%v", k.Interface())] = s.MapIndex(k).Interface()
		}
	case reflect.Struct:
		st := structs.New(s.Interface())
		for _, field := range st.Fields() {
			if !field.IsExported() {
				continue
			}
			if field.IsEmbedded() {
				for ek, ev := range getMap(field.Value()) {
					if _, ok := result[ek]; !ok {
						result[ek] = ev
					}
				}
				continue
			}

			tag := field.Tag("json")
			if tag == "-" {
				continue
			}
			if strings.Contains(tag, "omitempty") && field.IsZero() {
				continue
			}
			if tag == "" {
				tag = field.Name()
			}
			result[strings.Split(tag, ",")[0]] = field.Value()
		}
	}

	return result
}

func getIdentifier(v interface{}) (*string, interface{}) {
	str := structs.New(v)
	for _, field := range str.Fields() {
		if field.IsEmbedded() {
			return getIdentifier(field.Value())
		}
		if tag := field.Tag("json"); strings.Contains(tag, "identifier") {
			val := fmt.Sprintf("%v", field.Value())
			return &val, v
		}
	}

	return nil, nil
}

func compare(old, new interface{}, path []string, level CriticalityLevel) *Change {
	if reflect.DeepEqual(old, new) {
		return nil
	}

	return &Change{
		Path:        path,
		Type:        detectChangeType(old, new),
		Object:      detectObjectType(path),
		Criticality: level,
		Old:         old,
		New:         new,
	}
}

func detectObjectType(path []string) ChangeObject {
	for _, pair := range objectPaths {
		for pattern, object := range pair {
			if matchPath(path, pattern) {
				return object
			}
		}
	}

	return Other
}

var objectPaths = []map[string]ChangeObject{
	{"openrpc": OpenRPCVersion},
	{"info.version": OpenRPCVersion},
	{"info": SchemaInfo},
	{"servers": SchemaServers},

	{"methods.*.paramStructure": MethodParamStructure},
	{"methods.*.params.*.schema": MethodParamType},
	{"methods.*.params.*.$ref": MethodParamType},
	{"methods.*.params": MethodParam},

	{"methods.*.result.type": MethodResultType},
	{"methods.*.result.$ref": MethodResultType},
	{"methods.*.result": MethodResult},

	{"methods.*.errors": MethodError},

	{"methods": Method},

	{"components.schemas.*.type": ComponentsSchemaType},
	{"components.schemas.*.properties.*.type": ComponentsSchemaPropertyType},
	{"components.schemas.*.properties.*.schema": ComponentsSchemaPropertyType},
	{"components.schemas.*.properties.*.$ref": ComponentsSchemaPropertyType},
	{"components.schemas.*.properties": ComponentsSchemaProperty},
	{"components.schemas": ComponentsSchema},

	{"components.contentDescriptors.*.schema": ComponentsDescriptorType},
	{"components.contentDescriptors": ComponentsDescriptor},
}

func matchPath(path []string, pattern string) bool {
	pp := strings.Split(pattern, ".")
	if len(path) < len(pp) {
		return false
	}

	for i, p := range pp {
		if p == "*" {
			continue
		}
		if path[i] != p {
			return false
		}
	}

	return true
}

func detectChangeType(old, new interface{}) ChangeType {
	if isNil(old) {
		return Added
	} else if isNil(new) {
		return Removed
	}

	return Changed
}

func isNil(v interface{}) bool {
	return v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil())
}

func isTrue(v interface{}) bool {
	return reflect.ValueOf(v).Kind() == reflect.Bool && reflect.ValueOf(v).Bool() == true
}

func isStruct(v interface{}) bool {
	return structs.IsStruct(v)
}

func isSlice(v interface{}) bool {
	s := reflect.ValueOf(v)
	if s.Kind() == reflect.Ptr || s.Kind() == reflect.Interface {
		s = s.Elem()
	}

	return s.Kind() == reflect.Slice
}

func isMap(v interface{}) bool {
	s := reflect.ValueOf(v)
	if s.Kind() == reflect.Ptr || s.Kind() == reflect.Interface {
		s = s.Elem()
	}

	return s.Kind() == reflect.Map
}

func sameType(old, new interface{}) bool {
	return reflect.TypeOf(old) == reflect.TypeOf(new)
}

func last(path []string) string {
	if len(path) > 0 {
		return path[len(path)-1]
	}
	return ""
}

func contains(path []string, elements ...string) bool {
	for _, elem := range elements {
		if !funk.ContainsString(path, elem) {
			return false
		}
	}

	return true
}

func shift(path []string) (string, []string) {
	return path[0], path[1:]
}

func pop(path []string) (string, []string) {
	return path[len(path)-1], path[:len(path)-1]
}

func after(path []string, el string) string {
	for i, p := range path {
		if p == el && i+1 < len(path) {
			return path[i+1]
		}
	}

	return ""
}
