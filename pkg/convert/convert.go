package convert

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/ckanthony/gin-mcp/pkg/types"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// isDebugMode returns true if Gin is in debug mode
func isDebugMode() bool {
	return gin.Mode() == gin.DebugMode
}

// ConvertRoutesToTools converts Gin routes into a list of MCP Tools and an operation map.
func ConvertRoutesToTools(routes gin.RoutesInfo, registeredSchemas map[string]types.RegisteredSchemaInfo) ([]types.Tool, map[string]types.Operation) {
	ttools := make([]types.Tool, 0)
	operations := make(map[string]types.Operation)

	if isDebugMode() {
		log.Printf("Starting conversion of %d routes to MCP tools...", len(routes))
	}

	for _, route := range routes {
		// Simple operation ID generation (e.g., GET_users_id)
		operationID := strings.ToUpper(route.Method) + strings.ReplaceAll(strings.ReplaceAll(route.Path, "/", "_"), ":", "")

		if isDebugMode() {
			log.Printf("Processing route: %s %s -> OpID: %s", route.Method, route.Path, operationID)
		}

		// Generate schema for the tool's input
		inputSchema := generateInputSchema(route, registeredSchemas)

		// Create the tool definition
		tool := types.Tool{
			Name:        operationID,
			Description: fmt.Sprintf("Handler for %s %s", route.Method, route.Path), // Use route info for description
			InputSchema: inputSchema,
		}

		ttools = append(ttools, tool)
		operations[operationID] = types.Operation{
			Method: route.Method,
			Path:   route.Path,
		}
	}

	if isDebugMode() {
		log.Printf("Finished route conversion. Generated %d tools.", len(ttools))
	}

	return ttools, operations
}

// PathParamRegex is used to find path parameters like :id or *action
var PathParamRegex = regexp.MustCompile(`[:\*]([a-zA-Z0-9_]+)`)

// generateInputSchema creates the JSON schema for the tool's input parameters.
// This is a simplified version using basic reflection and not an external library.
func generateInputSchema(route gin.RouteInfo, registeredSchemas map[string]types.RegisteredSchemaInfo) *types.JSONSchema {
	// Base schema structure
	schema := &types.JSONSchema{
		Type:       "object",
		Properties: make(map[string]*types.JSONSchema),
		Required:   make([]string, 0),
	}
	properties := schema.Properties
	required := schema.Required

	// 1. Extract Path Parameters
	matches := PathParamRegex.FindAllStringSubmatch(route.Path, -1)
	for _, match := range matches {
		if len(match) > 1 {
			paramName := match[1]
			properties[paramName] = &types.JSONSchema{
				Type:        "string",
				Description: fmt.Sprintf("Path parameter: %s", paramName),
			}
			required = append(required, paramName) // Path params are always required
		}
	}

	// 2. Incorporate Registered Query and Body Types
	schemaKey := route.Method + " " + route.Path
	if schemaInfo, exists := registeredSchemas[schemaKey]; exists {
		if isDebugMode() {
			log.Printf("Using registered schema for %s", schemaKey)
		}

		// Reflect Query Parameters (if applicable for method and type exists)
		if (route.Method == "GET" || route.Method == "DELETE") && schemaInfo.QueryType != nil {
			reflectAndAddProperties(schemaInfo.QueryType, properties, &required, "query")
		}

		// Reflect Body Parameters (if applicable for method and type exists)
		if (route.Method == "POST" || route.Method == "PUT" || route.Method == "PATCH") && schemaInfo.BodyType != nil {
			reflectAndAddProperties(schemaInfo.BodyType, properties, &required, "body")
		}
	}

	// Update the required slice in the main schema
	schema.Required = required

	// If no properties were added (beyond path params), handle appropriately.
	// Depending on the spec, an empty properties object might be required.
	// if len(properties) == 0 { // Keep properties map even if empty
	// 	// Return schema with empty properties
	// 	return schema
	// }

	return schema
}

// reflectAndAddProperties uses basic reflection to add properties to the schema.
func reflectAndAddProperties(goType interface{}, properties map[string]*types.JSONSchema, required *[]string, source string) {
	if goType == nil {
		return // Handle nil input gracefully
	}
	t := types.ReflectType(reflect.TypeOf(goType)) // Use helper from types pkg
	if t == nil || t.Kind() != reflect.Struct {
		if isDebugMode() {
			log.Printf("Skipping schema generation for non-struct type: %v (%s)", reflect.TypeOf(goType), source)
		}
		return
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		formTag := field.Tag.Get("form")             // Used for query params often
		jsonschemaTag := field.Tag.Get("jsonschema") // Basic support

		fieldName := field.Name // Default to field name
		ignoreField := false

		// Determine field name from tags (prefer json, then form)
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] == "-" {
				ignoreField = true
			} else {
				fieldName = parts[0]
			}
			if len(parts) > 1 && parts[1] == "omitempty" {
				// omitempty = true // Variable removed
			}
		} else if formTag != "" {
			parts := strings.Split(formTag, ",")
			if parts[0] == "-" {
				ignoreField = true
			} else {
				fieldName = parts[0]
			}
			// form tag doesn't typically have omitempty in the same way
		}

		if ignoreField || !field.IsExported() {
			continue
		}

		propSchema := &types.JSONSchema{}

		// Basic type mapping
		switch field.Type.Kind() {
		case reflect.String:
			propSchema.Type = "string"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			propSchema.Type = "integer"
		case reflect.Float32, reflect.Float64:
			propSchema.Type = "number"
		case reflect.Bool:
			propSchema.Type = "boolean"
		case reflect.Slice, reflect.Array:
			propSchema.Type = "array"
			// TODO: Implement items schema based on element type
			propSchema.Items = &types.JSONSchema{Type: "string"} // Placeholder
		case reflect.Map:
			propSchema.Type = "object"
			// TODO: Implement properties schema based on map key/value types
		case reflect.Struct:
			propSchema.Type = "object"
			// Potentially recurse, but keep simple for now
		default:
			propSchema.Type = "string" // Default fallback
		}

		// Basic 'required' and 'description' handling from jsonschema tag
		isRequired := false // Default to not required
		if jsonschemaTag != "" {
			parts := strings.Split(jsonschemaTag, ",")
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed == "required" {
					isRequired = true
				} else if strings.HasPrefix(trimmed, "description=") {
					propSchema.Description = strings.TrimPrefix(trimmed, "description=")
				}
				// TODO: Add more tag parsing (minimum, maximum, enum, etc.)
			}
		}

		// Add to properties map
		properties[fieldName] = propSchema

		// Add to required list if necessary
		if isRequired {
			*required = append(*required, fieldName)
		}
	}
}
