package main

import (
	"encoding/json"
	"fmt"
	"github.com/thoas/go-funk"
	"io/ioutil"
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
	SchemaVersion ChangeObject = "SCHEMA_VERSION"
	SchemaServers ChangeObject = "SCHEMA_SERVERS"

	Method               ChangeObject = "METHOD"
	MethodTags           ChangeObject = "METHOD_TAGS"
	MethodSummary        ChangeObject = "METHOD_SUMMARY"
	MethodParamStructure ChangeObject = "METHOD_PARAM_STRUCTURE"

	MethodParam         ChangeObject = "METHOD_PARAM"
	MethodParamRequired ChangeObject = "METHOD_PARAM_REQUIRED"
	MethodParamType     ChangeObject = "METHOD_PARAM_TYPE" // type + ref + items type + items ref
	MethodParamDesc     ChangeObject = "METHOD_PARAM_DESC"

	MethodResultName ChangeObject = "METHOD_RESULT"
	MethodResultDesc ChangeObject = "METHOD_RESULT_DESC"
	MethodResultType ChangeObject = "METHOD_RESULT_TYPE" // schema type + ref

	MethodError        ChangeObject = "METHOD_ERROR"
	MethodErrorMessage ChangeObject = "METHOD_ERROR_MSG"

	ComponentsSchema             ChangeObject = "COMPONENTS_SCHEMA"
	ComponentsSchemaProperty     ChangeObject = "COMPONENTS_SCHEMA_PROPERTY"
	ComponentsSchemaPropertyDesc ChangeObject = "COMPONENTS_SCHEMA_PROPERTY_REQUIRED"
	ComponentsSchemaPropertyType ChangeObject = "COMPONENTS_SCHEMA_PROPERTY_TYPE" // type + ref + items type + items ref

	ComponentsDescriptor        ChangeObject = "COMPONENTS_DESCRIPTOR"
	ComponentsDescriptorSummary ChangeObject = "COMPONENTS_DESCRIPTOR_SUMMARY"
	ComponentsDescriptorType    ChangeObject = "COMPONENTS_DESCRIPTOR_TYPE"

	Other ChangeObject = "OTHER"
)

type comparerFunc func(path []string, from, to interface{}) *Change

type changePath struct {
	pattern string
	comp    comparerFunc
}

var changePaths = []changePath{
	{
		pattern: "openrpc",
		comp:    openrpcCompare,
	},
	{
		pattern: "info.version",
		comp:    plainCompare(SchemaVersion),
	},
	{
		pattern: "info",
		comp:    plainCompare(SchemaInfo),
	},
	{
		pattern: "servers",
		comp:    plainCompare(SchemaServers),
	},
	{
		pattern: "methods.*.params.*.schema",
		comp:    typeCompare(MethodParamType),
	},
	{
		pattern: "methods.*.params",
		comp:    paramsCompare,
	},
	{
		pattern: "methods.*.paramsStructure",
		comp:    paramStructureCompare,
	},
	{
		pattern: "methods.*.tags",
		comp:    plainCompare(MethodTags),
	},
	{
		pattern: "methods.*.summary",
		comp:    plainCompare(MethodSummary),
	},
	{
		pattern: "methods.*.result",
		comp:    methodResultCompare,
	},
	{
		pattern: "methods.*.errors",
		comp:    methodErrorCompare,
	},
	{
		pattern: "methods",
		comp:    strictCompare(Method),
	},
	{
		pattern: "components.schemas.*.properties.*.*",
		comp:    schemaPropertyCompare,
	},
	{
		pattern: "components.schemas.*.properties.*",
		comp:    plainCompare(ComponentsSchemaProperty),
	},
	{
		pattern: "components.schemas",
		comp:    strictCompare(ComponentsSchema),
	},
	{
		pattern: "components.contentDescriptors.*.*",
		comp:    schemaDescriptorCompare,
	},
	{
		pattern: "components.contentDescriptors.*",
		comp:    plainCompare(ComponentsDescriptor),
	},
	{
		pattern: "components.contentDescriptors",
		comp:    strictCompare(ComponentsSchema),
	},
}

type Change struct {
	Path        []string         `json:"path"`
	Type        ChangeType       `json:"type"`
	Object      ChangeObject     `json:"object"`
	Criticality CriticalityLevel `json:"criticality"`
	From        interface{}
	To          interface{}

	// TODO Add related paths to definitions/schemas diffs
	//Related     []string `json:"related"`
}

func (c *Change) String() string {
	from, _ := json.Marshal(c.From)
	to, _ := json.Marshal(c.To)

	switch c.Type {
	case Added:
		return fmt.Sprintf("added %s: %s", stringPath(c.Path), to)
	case Removed:
		return fmt.Sprintf("removed %s: %s", stringPath(c.Path), from)
	default:
		return fmt.Sprintf("changed %s from %s to %s", stringPath(c.Path), from, to)
	}
}

func stringPath(path []string) string {
	if len(path) == 1 {
		return path[0]
	}

	section, path := shift(path)
	element, path := pop(path)
	middle := ""
	if len(path) != 0 {
		middle = fmt.Sprintf(": %s", strings.Join(path, "."))
	}

	switch section {
	// special path print for methods
	case "methods":
		if contains(path, "params") {
			if last(path) == "params" {
				element = fmt.Sprintf("%s param", element)
				middle = fmt.Sprintf(": %s", path[0])
			}
			if len(path) > 2 {
				middle = fmt.Sprintf(": %s(%s)", path[0], path[2])
			}
		}
		if last(path) == "errors" {
			element = fmt.Sprintf("%s error", element)
			middle = fmt.Sprintf(": %s", path[0])
		}
	}

	return fmt.Sprintf("%s at %s%s", element, section, middle)
}

type Diff struct {
	Criticality CriticalityLevel
	Changes     []Change
}

func NewDiff(oldJSON, newJSON []byte) (*Diff, error) {
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
		Changes:     []Change{},
	}

	compareRecursive(diff)(oldSchema, newSchema, []string{})

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

func NewDiffFiles(oldFilename, newFilename string) (*Diff, error) {
	oldJSON, err := ioutil.ReadFile(oldFilename)
	if err != nil {
		return nil, fmt.Errorf("read old json schema error: %w", err)
	}

	newJSON, err := ioutil.ReadFile(newFilename)
	if err != nil {
		return nil, fmt.Errorf("read new json schema error: %w", err)
	}

	return NewDiff(oldJSON, newJSON)
}

func NewDiffStrings(oldJSON, newJSON string) (*Diff, error) {
	return NewDiff([]byte(oldJSON), []byte(newJSON))
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

func compareRecursive(diff *Diff) func(old, new interface{}, path []string) {
	return func(old, new interface{}, path []string) {
		if !sameType(old, new) {
			diff.Changes = append(diff.Changes, *compare(path, old, new))
		}

		// embed simple types
		if okOld, oldE := getEmbedSimpleType(old); okOld {
			if okNew, newE := getEmbedSimpleType(new); okNew {
				if change := compare(path, oldE, newE); change != nil {
					diff.Changes = append(diff.Changes, *change)
				}
			}
		}

		if (isStruct(old) && isStruct(new)) || (isMap(old) && isMap(new)) || (isSlice(old) && isSlice(new)) {
			oldMap := getMap(old)
			newMap := getMap(new)

			for oldFieldName, oldFieldVal := range oldMap {
				if newFieldVal, ok := newMap[oldFieldName]; ok {
					compareRecursive(diff)(oldFieldVal, newFieldVal, append(path, oldFieldName))

					delete(newMap, oldFieldName)
				} else {
					diff.Changes = append(diff.Changes, *compare(append(path, oldFieldName), oldFieldVal, nil))
				}
			}

			for leftFieldName, leftFieldVal := range newMap {
				diff.Changes = append(diff.Changes, *compare(append(path, leftFieldName), nil, leftFieldVal))
			}

			return
		}

		if change := compare(path, old, new); change != nil {
			diff.Changes = append(diff.Changes, *change)
		}
	}
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

func compare(path []string, old, new interface{}) *Change {
	if reflect.DeepEqual(old, new) {
		return nil
	}

	for _, c := range changePaths {
		if matchPath(path, c.pattern) {
			return c.comp(path, old, new)
		}
	}

	return plainCompare(Other)(path, old, new)
}

func NewChange(path []string, old, new interface{}) *Change {
	for _, c := range changePaths {
		if matchPath(path, c.pattern) {
			return c.comp(path, old, new)
		}
	}

	return plainCompare(Other)(path, old, new)
}

func plainCompare(object ChangeObject) comparerFunc {
	return func(path []string, from, to interface{}) *Change {
		var changeType ChangeType

		if isNil(from) {
			changeType = Added
		} else if isNil(to) {
			changeType = Removed
		} else {
			changeType = Changed
		}

		return &Change{
			Path:        path,
			Type:        changeType,
			Object:      object,
			Criticality: NonBreaking, // everything is not breaking unless out explicitly change it
			From:        from,
			To:          to,
		}
	}
}

func strictCompare(object ChangeObject) comparerFunc {
	return func(path []string, from, to interface{}) *Change {
		change := plainCompare(object)(path, from, to)
		switch change.Type {
		case Removed, Changed:
			change.Criticality = Breaking
		}

		return change
	}
}

func openrpcCompare(path []string, from, to interface{}) *Change {
	change := plainCompare(OpenRPCVersion)(path, from, to)
	change.Criticality = Breaking

	return change
}

func typeCompare(object ChangeObject) comparerFunc {
	return func(path []string, from, to interface{}) *Change {
		// TODO advanced type comparison

		change := plainCompare(object)(path, from, to)
		change.Criticality = Breaking

		return change
	}
}

func paramsCompare(path []string, from, to interface{}) *Change {
	if last(path) == "required" {
		level := NonBreaking
		if !isTrue(from) && isTrue(to) {
			level = Breaking
		}

		return &Change{
			Path:        path,
			Type:        Changed,
			Object:      MethodParamRequired,
			Criticality: level,
			From:        from,
			To:          to,
		}
	}

	change := plainCompare(MethodParam)(path, from, to)
	if change.Type == Added && structs.IsStruct(to) {
		if toV := structs.Map(to); isTrue(toV["Required"]) {
			change.Criticality = Breaking
		}
	}
	return change
}

func paramStructureCompare(path []string, from, to interface{}) *Change {
	var criticality CriticalityLevel
	toV, ok := to.(openrpc.MethodObjectParamStructure)
	if !ok || toV == "" {
		toV = openrpc.MethodObjectParamStructureEnum2
	}

	for {
		if toV == openrpc.MethodObjectParamStructureEnum2 {
			criticality = NonBreaking
			break
		}

		fromV := from.(openrpc.MethodObjectParamStructure)
		if fromV == openrpc.MethodObjectParamStructureEnum2 {
			criticality = Dangerous
			break
		}

		if fromV != toV {
			criticality = Breaking
			break
		}
		break
	}

	return &Change{
		Path:        path,
		Type:        Changed,
		Object:      MethodParamStructure,
		Criticality: criticality,
		From:        from,
		To:          to,
	}
}

func methodResultCompare(path []string, from, to interface{}) *Change {
	// description is changed
	if last(path) == "description" {
		return plainCompare(MethodResultDesc)(path, from, to)
	}
	// name is changed
	if last(path) == "name" {
		return strictCompare(MethodResultName)(path, from, to)
	}

	// TODO advanced type comparison
	return strictCompare(MethodResultType)(path, from, to)
}

func methodErrorCompare(path []string, from, to interface{}) *Change {
	// message is changed
	if last(path) == "message" {
		return plainCompare(MethodErrorMessage)(path, from, to)
	}

	change := plainCompare(MethodError)(path, from, to)
	if change.Type == Added {
		change.Criticality = Dangerous
	}

	return change
}

func schemaPropertyCompare(path []string, from, to interface{}) *Change {
	// description is changed
	if last(path) == "description" {
		return plainCompare(ComponentsSchemaPropertyDesc)(path, from, to)
	}

	// TODO make different comparison for output objects an input objects
	return strictCompare(ComponentsSchemaPropertyType)(path, from, to)
}

func schemaDescriptorCompare(path []string, from, to interface{}) *Change {
	// summary is changed
	if last(path) == "summary" {
		return plainCompare(ComponentsDescriptorSummary)(path, from, to)
	}

	// TODO make different comparison for output objects an input objects
	return strictCompare(ComponentsDescriptorType)(path, from, to)
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
