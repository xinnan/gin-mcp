package types

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Tests for RawMessage ---

func TestRawMessage_MarshalJSON(t *testing.T) {
	// Test nil RawMessage
	var nilMsg RawMessage
	data, err := json.Marshal(nilMsg)
	assert.NoError(t, err)
	assert.Equal(t, "null", string(data), "Marshaling nil RawMessage should produce 'null'")

	// Test non-nil RawMessage
	rawJson := json.RawMessage(`{"key":"value","num":123}`)
	msg := RawMessage(rawJson)
	data, err = json.Marshal(msg)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key":"value","num":123}`, string(data), "Marshaling RawMessage should preserve original JSON")

	// Test marshaling within a struct
	type ContainingStruct struct {
		Field1 string     `json:"field1"`
		Raw    RawMessage `json:"raw"`
	}
	container := ContainingStruct{
		Field1: "test",
		Raw:    msg,
	}
	data, err = json.Marshal(container)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"field1":"test","raw":{"key":"value","num":123}}`, string(data))
}

func TestRawMessage_UnmarshalJSON(t *testing.T) {
	jsonData := `{"key":"value","num":123}`
	var msg RawMessage

	err := json.Unmarshal([]byte(jsonData), &msg)
	assert.NoError(t, err)
	assert.Equal(t, jsonData, string(msg), "Unmarshaling should copy the raw JSON data")

	// Test unmarshaling null
	jsonDataNull := `null`
	var msgNull RawMessage
	err = json.Unmarshal([]byte(jsonDataNull), &msgNull)
	assert.NoError(t, err)
	assert.Equal(t, jsonDataNull, string(msgNull), "Unmarshaling null should work") // RawMessage becomes []byte("null")

	// Test unmarshaling into a nil pointer (should error)
	var nilPtr *RawMessage
	err = json.Unmarshal([]byte(jsonData), nilPtr) // Pass nil pointer
	assert.Error(t, err, "Unmarshaling into a nil pointer should error")

	// Test unmarshaling within a struct
	type ContainingStructUnmarshal struct {
		Field1 string     `json:"field1"`
		Raw    RawMessage `json:"raw"`
	}
	fullJson := `{"field1":"test","raw":{"nested":true}}`
	var container ContainingStructUnmarshal
	err = json.Unmarshal([]byte(fullJson), &container)
	assert.NoError(t, err)
	assert.Equal(t, "test", container.Field1)
	assert.JSONEq(t, `{"nested":true}`, string(container.Raw))
}

// --- Tests for GetSchema ---

type SimpleStruct struct {
	Name string `json:"name" jsonschema:"required,description=The name"`
	Age  int    `json:"age,omitempty" jsonschema:"minimum=0"`
}

type ComplexStruct struct {
	ID      string        `json:"id" jsonschema:"required"`
	Simple  SimpleStruct  `json:"simple_data"`
	Values  []float64     `json:"values"`
	Ignored string        `json:"-"`
	FormTag string        `form:"form_field"` // Should be picked up if no json tag
	Pointer *SimpleStruct `json:"pointer_data,omitempty"`
}

func TestGetSchema_Simple(t *testing.T) {
	schema := GetSchema(SimpleStruct{})

	require.NotNil(t, schema)
	assert.Equal(t, "object", schema["type"])

	require.Contains(t, schema, "properties")
	properties, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Len(t, properties, 2)

	// Check Name field
	require.Contains(t, properties, "name")
	nameSchema, ok := properties["name"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "string", nameSchema["type"])
	assert.Equal(t, "The name", nameSchema["description"])

	// Check Age field
	require.Contains(t, properties, "age")
	ageSchema, ok := properties["age"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "integer", ageSchema["type"])
	assert.Equal(t, float64(0), ageSchema["minimum"])

	// Check required fields
	require.Contains(t, schema, "required")
	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Len(t, required, 1)
	assert.Contains(t, required, "name")
	assert.NotContains(t, required, "age") // omitempty implies not required by default
}

func TestGetSchema_Complex(t *testing.T) {
	// Test with pointer type as well
	schemaPtr := GetSchema(&ComplexStruct{})
	schemaVal := GetSchema(ComplexStruct{})

	assert.Equal(t, schemaVal, schemaPtr, "Schema should be the same for value and pointer")

	schema := schemaVal // Use one for checks
	require.NotNil(t, schema)
	assert.Equal(t, "object", schema["type"])

	require.Contains(t, schema, "properties")
	properties, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok)
	// ID, Simple, Values, FormTag, Pointer -> 5 fields
	assert.Len(t, properties, 5, "Should have 5 properties (ID, Simple, Values, FormTag, Pointer)")

	// Check ID
	require.Contains(t, properties, "id")
	idSchema, _ := properties["id"].(map[string]interface{})
	assert.Equal(t, "string", idSchema["type"])

	// Check Simple (nested struct - currently just object)
	require.Contains(t, properties, "simple_data")
	simpleSchema, _ := properties["simple_data"].(map[string]interface{})
	assert.Equal(t, "object", simpleSchema["type"], "Nested structs are represented as 'object' for now")

	// Check Values (slice - currently just array)
	require.Contains(t, properties, "values")
	valuesSchema, _ := properties["values"].(map[string]interface{})
	assert.Equal(t, "array", valuesSchema["type"], "Slices are represented as 'array'")
	// TODO: Check items when implemented

	// Check FormTag field
	require.Contains(t, properties, "form_field")
	formSchema, _ := properties["form_field"].(map[string]interface{})
	assert.Equal(t, "string", formSchema["type"])

	// Check Pointer field (nested struct pointer - currently just object)
	require.Contains(t, properties, "pointer_data")
	pointerSchema, _ := properties["pointer_data"].(map[string]interface{})
	assert.Equal(t, "object", pointerSchema["type"], "Pointer to structs are represented as 'object' for now")

	// Check Ignored field is not present
	assert.NotContains(t, properties, "-")
	assert.NotContains(t, properties, "Ignored")

	// Check required fields
	require.Contains(t, schema, "required")
	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Len(t, required, 1)
	assert.Contains(t, required, "id") // Only ID has jsonschema:required
}

func TestGetSchema_NilInput(t *testing.T) {
	schema := GetSchema(nil)
	assert.Nil(t, schema, "GetSchema(nil) should return nil")

	var nilPtr *SimpleStruct
	schema = GetSchema(nilPtr)
	assert.Nil(t, schema, "GetSchema(nil struct pointer) should return nil") // ReflectType handles this
}

func TestGetSchema_NonStructInput(t *testing.T) {
	// Test with basic types - should return default object schema
	assert.Equal(t, map[string]interface{}{"type": "object"}, GetSchema(123))
	assert.Equal(t, map[string]interface{}{"type": "object"}, GetSchema("hello"))
	arr := []int{1}
	assert.Equal(t, map[string]interface{}{"type": "object"}, GetSchema(arr))
	m := map[string]int{}
	assert.Equal(t, map[string]interface{}{"type": "object"}, GetSchema(m))
}

// --- Tests for getUnderlyingType ---

func TestGetUnderlyingType(t *testing.T) {
	var s SimpleStruct
	var ps *SimpleStruct
	var pps **SimpleStruct

	t_s, k_s := getUnderlyingType(reflect.TypeOf(s))
	assert.Equal(t, reflect.TypeOf(s), t_s)
	assert.Equal(t, reflect.Struct, k_s)

	t_ps, k_ps := getUnderlyingType(reflect.TypeOf(ps))
	assert.Equal(t, reflect.TypeOf(s), t_ps) // Should be element type
	assert.Equal(t, reflect.Struct, k_ps)    // Kind of the element type

	// Test double pointer - should only dereference once
	t_pps, k_pps := getUnderlyingType(reflect.TypeOf(pps))
	assert.Equal(t, reflect.TypeOf(ps), t_pps) // Should be *SimpleStruct
	assert.Equal(t, reflect.Ptr, k_pps)        // Kind should be Ptr

	// Test non-pointer
	var i int
	t_i, k_i := getUnderlyingType(reflect.TypeOf(i))
	assert.Equal(t, reflect.TypeOf(i), t_i)
	assert.Equal(t, reflect.Int, k_i)
}

// --- Tests for ReflectType ---

func TestReflectType(t *testing.T) {
	var s SimpleStruct
	var ps *SimpleStruct
	var pps **SimpleStruct
	var sl []SimpleStruct
	var psl *[]SimpleStruct
	var pslp *[]*SimpleStruct
	var i int
	var pi *int

	// Nil input
	assert.Nil(t, ReflectType(nil), "ReflectType(nil) should return nil")

	// Basic types
	assert.Equal(t, reflect.TypeOf(s), ReflectType(reflect.TypeOf(s)), "Struct type")
	assert.Equal(t, reflect.TypeOf(i), ReflectType(reflect.TypeOf(i)), "Int type")

	// Pointers
	assert.Equal(t, reflect.TypeOf(s), ReflectType(reflect.TypeOf(ps)), "Pointer to struct")
	assert.Equal(t, reflect.TypeOf(s), ReflectType(reflect.TypeOf(pps)), "Double pointer to struct")
	assert.Equal(t, reflect.TypeOf(i), ReflectType(reflect.TypeOf(pi)), "Pointer to int")

	// Slices
	assert.Equal(t, reflect.TypeOf(s), ReflectType(reflect.TypeOf(sl)), "Slice of struct")
	assert.Equal(t, reflect.TypeOf(s), ReflectType(reflect.TypeOf(psl)), "Pointer to slice of struct")
	assert.Equal(t, reflect.TypeOf(s), ReflectType(reflect.TypeOf(pslp)), "Pointer to slice of pointer to struct")

	// Edge case: Slice of pointers
	var slp []*SimpleStruct
	assert.Equal(t, reflect.TypeOf(s), ReflectType(reflect.TypeOf(slp)), "Slice of pointer to struct")

}

// --- Test Constants/Basic Structs (Presence checks) ---

func TestConstantsAndStructs(t *testing.T) {
	// Just ensure constants exist
	assert.Equal(t, ContentTypeText, ContentType("text"))
	assert.Equal(t, ContentTypeJSON, ContentType("json"))
	assert.Equal(t, ContentTypeError, ContentType("error"))
	assert.Equal(t, ContentTypeImage, ContentType("image"))

	// Ensure structs can be instantiated (compile-time check mostly)
	_ = MCPMessage{}
	_ = Tool{}
	_ = Operation{}
	_ = RegisteredSchemaInfo{}
	_ = JSONSchema{}

}
