package main

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	server "github.com/ckanthony/gin-mcp"
	"github.com/gin-gonic/gin"
)

// Product represents a product in our store
type Product struct {
	ID          int      `json:"id" jsonschema:"readOnly"`
	Name        string   `json:"name" jsonschema:"required,description=Name of the product"`
	Description string   `json:"description,omitempty" jsonschema:"description=Detailed description of the product"`
	Price       float64  `json:"price" jsonschema:"required,minimum=0,description=Price in USD"`
	Tags        []string `json:"tags,omitempty" jsonschema:"description=Categories or labels for the product"`
	IsEnabled   bool     `json:"is_enabled" jsonschema:"required,description=Whether the product is available for purchase"`
}

// UpdateProductRequest represents the request body for updating a product
type UpdateProductRequest struct {
	Name        string   `json:"name" jsonschema:"required,description=New name of the product"`
	Description string   `json:"description,omitempty" jsonschema:"description=New description of the product"`
	Price       float64  `json:"price" jsonschema:"required,minimum=0,description=New price in USD"`
	Tags        []string `json:"tags,omitempty" jsonschema:"description=New categories or labels"`
	IsEnabled   bool     `json:"is_enabled" jsonschema:"required,description=New availability status"`
}

// ListProductsParams defines query parameters for product listing and searching
type ListProductsParams struct {
	// Search parameters
	Query string `form:"q" json:"q,omitempty" jsonschema:"description=Search query string for product name and description"`

	// Filter parameters
	MinPrice float64 `form:"minPrice" json:"minPrice,omitempty" jsonschema:"description=Minimum price filter"`
	MaxPrice float64 `form:"maxPrice" json:"maxPrice,omitempty" jsonschema:"description=Maximum price filter"`
	Tag      string  `form:"tag" json:"tag,omitempty" jsonschema:"description=Filter by specific tag"`
	Enabled  *bool   `form:"enabled" json:"enabled,omitempty" jsonschema:"description=Filter by availability status"`

	// Pagination parameters
	Page  int `form:"page,default=1" json:"page,omitempty" jsonschema:"description=Page number,minimum=1,default=1"`
	Limit int `form:"limit,default=10" json:"limit,omitempty" jsonschema:"description=Items per page,minimum=1,maximum=100,default=10"`

	// Sorting parameters
	SortBy string `form:"sortBy,default=id" json:"sortBy,omitempty" jsonschema:"description=Field to sort by,enum=id,enum=price"`
	Order  string `form:"order,default=asc" json:"order,omitempty" jsonschema:"description=Sort order,enum=asc,enum=desc"`
}

// In-memory store
var (
	products = make(map[int]*Product)
	nextID   = 1
)

// Initialize sample products
func init() {
	products[nextID] = &Product{
		ID:          nextID,
		Name:        "Quantum Bug Repellent",
		Description: "Keeps bugs out of your code using quantum entanglement. Warning: May cause Schrödinger's bugs",
		Price:       15.99,
		Tags:        []string{"programming", "quantum", "debugging"},
		IsEnabled:   true,
	}
	nextID++

	products[nextID] = &Product{
		ID:          nextID,
		Name:        "HTTP Status Cat Poster",
		Description: "A poster featuring cats representing HTTP status codes. 404 Cat Not Found included!",
		Price:       19.99,
		Tags:        []string{"web", "cats", "decoration"},
		IsEnabled:   true,
	}
	nextID++

	products[nextID] = &Product{
		ID:          nextID,
		Name:        "Rubber Duck Debug Force™",
		Description: "Special forces rubber duck trained in advanced debugging techniques. Has PhD in Computer Science",
		Price:       42.42,
		Tags:        []string{"debugging", "rubber-duck", "consultant"},
		IsEnabled:   true,
	}
	nextID++

	products[nextID] = &Product{
		ID:          nextID,
		Name:        "Infinite Loop Coffee Maker",
		Description: "Keeps making coffee until stack overflow. Comes with catch{} block cup holder",
		Price:       99.99,
		Tags:        []string{"coffee", "programming", "kitchen"},
		IsEnabled:   true,
	}
	nextID++
}

func main() {
	gin.SetMode(gin.DebugMode)

	// Use Default() which includes logger and recovery middleware
	r := gin.Default()

	// Register API routes
	registerRoutes(r)

	// Initialize and configure MCP server
	configureMCP(r)

	// Start the server
	r.Run(":8080")
}

// Register API routes
func registerRoutes(r *gin.Engine) {
	// CRUD endpoints
	r.GET("/products", listProducts)
	r.GET("/products/:id", getProduct)
	r.POST("/products", createProduct)
	r.PUT("/products/:id", updateProduct)
	r.DELETE("/products/:id", deleteProduct)

	// Search endpoint
	r.GET("/products/search", searchProducts)
}

// Configure MCP server
func configureMCP(r *gin.Engine) {
	mcp := server.New(r, &server.Config{
		Name:        "Gaming Store API",
		Description: "RESTful API for managing gaming products",
		BaseURL:     "http://localhost:8080",
	})

	// Register request schemas for MCP
	mcp.RegisterSchema("GET", "/products", ListProductsParams{}, nil)
	mcp.RegisterSchema("POST", "/products", nil, Product{})
	mcp.RegisterSchema("PUT", "/products/:id", nil, UpdateProductRequest{})

	// Mount MCP endpoint
	mcp.Mount("/mcp")
}

// Handler functions

func listProducts(c *gin.Context) {
	var params ListProductsParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate and normalize pagination params
	params.Page = max(params.Page, 1)
	params.Limit = clamp(params.Limit, 1, 100)

	var result []*Product

	// Apply filters
	for _, product := range products {
		if !applyFilters(product, &params) {
			continue
		}
		result = append(result, product)
	}

	// Sort results
	sortProducts(&result, params.SortBy, params.Order)

	// Apply pagination
	paginatedResult := paginateResults(result, params.Page, params.Limit)

	// Return response with metadata
	c.JSON(http.StatusOK, gin.H{
		"products": paginatedResult,
		"meta": gin.H{
			"page":       params.Page,
			"limit":      params.Limit,
			"total":      len(result),
			"totalPages": (len(result) + params.Limit - 1) / params.Limit,
		},
	})
}

func searchProducts(c *gin.Context) {
	query := strings.ToLower(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	var results []*Product
	for _, product := range products {
		if matchesSearchQuery(product, query) {
			results = append(results, product)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"products": results,
		"meta": gin.H{
			"total": len(results),
			"query": query,
		},
	})
}

func getProduct(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if product, exists := products[id]; exists {
		c.JSON(http.StatusOK, product)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
}

func createProduct(c *gin.Context) {
	var product Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product.ID = nextID
	nextID++

	products[product.ID] = &product
	c.JSON(http.StatusCreated, product)
}

func updateProduct(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if _, exists := products[id]; exists {
		var updateReq UpdateProductRequest
		if err := c.ShouldBindJSON(&updateReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		updatedProduct := &Product{
			ID:          id,
			Name:        updateReq.Name,
			Description: updateReq.Description,
			Price:       updateReq.Price,
			Tags:        updateReq.Tags,
			IsEnabled:   updateReq.IsEnabled,
		}

		products[id] = updatedProduct
		c.JSON(http.StatusOK, updatedProduct)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
}

func deleteProduct(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if _, exists := products[id]; exists {
		delete(products, id)
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
}

// Helper functions

func applyFilters(product *Product, params *ListProductsParams) bool {
	// Price filter
	if params.MinPrice > 0 && product.Price < params.MinPrice {
		return false
	}
	if params.MaxPrice > 0 && product.Price > params.MaxPrice {
		return false
	}

	// Tag filter
	if params.Tag != "" && !containsTag(product.Tags, params.Tag) {
		return false
	}

	// Enabled filter
	if params.Enabled != nil && product.IsEnabled != *params.Enabled {
		return false
	}

	return true
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func matchesSearchQuery(product *Product, query string) bool {
	return strings.Contains(strings.ToLower(product.Name), query) ||
		strings.Contains(strings.ToLower(product.Description), query)
}

func sortProducts(products *[]*Product, sortBy, order string) {
	sortBy = strings.ToLower(sortBy)
	order = strings.ToLower(order)

	sort.Slice(*products, func(i, j int) bool {
		a := (*products)[i]
		b := (*products)[j]

		var comparison bool
		switch sortBy {
		case "price":
			comparison = a.Price < b.Price
		default:
			comparison = a.ID < b.ID
		}

		return comparison != (order == "desc")
	})
}

func paginateResults(results []*Product, page, limit int) []*Product {
	start := (page - 1) * limit
	if start >= len(results) {
		return []*Product{}
	}

	end := min(start+limit, len(results))
	return results[start:end]
}

// Utility functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
