package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Comment struct {
	gorm.Model
	URI        string    `json:"uri"`
	URIEncoded string    `json:"uri_encoded"`
	Nickname   string    `json:"nickname"`
	Email      string    `json:"email"`
	Title      string    `json:"title,omitempty"`
	Content    string    `json:"content"`
	InsertedAt time.Time `json:"inserted_at"`
}

type CommentRequest struct {
	URI        string `json:"uri"`
	URIEncoded string `json:"uri_encoded"`
	Nickname   string `json:"nickname"`
	Email      string `json:"email"`
	Title      string `json:"title"`
	Content    string `json:"content"`
}

type PaginatedResponse struct {
	Comments   []Comment `json:"comments"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}


var GlobalCtx map[string]any = make(map[string]any)
func main() {
	db, err := gorm.Open(sqlite.Open("main.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	ctx := context.Background()
	GlobalCtx["ctx"] = &ctx
	GlobalCtx["db"] = db

	// Migrate the schema
	db.AutoMigrate(&Comment{})

	http.HandleFunc("/comments", commentsHandler)

	fmt.Println("Server starting on http://localhost:8000")
	fmt.Println("Endpoints:")
	fmt.Println("  GET  /comments?page=1&page_size=10")
	fmt.Println("  POST /comments")

	log.Fatal(http.ListenAndServe(":8000", nil))
}

func commentsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		handleGetComments(w, r)
	case http.MethodPost:
		handlePostComment(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
	}
}
func EncodeHex(s string) string {
	return hex.EncodeToString([]byte(s))
}

func DecodeHex(hexStr string) (string, error) {
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func GetURI(uri string, uriEncoded string, getEncoded bool) string {
	if getEncoded {
		if uriEncoded != "" {
			return uriEncoded
		}
		return EncodeHex(uri)
	}
	if uri != "" {
		return uri
	}
	s, err := DecodeHex(uriEncoded)
	if err != nil {
		return ""
	}
	return s
}

func handleGetComments(w http.ResponseWriter, r *http.Request) {
	db := GlobalCtx["db"].(*gorm.DB)
	ctx := GlobalCtx["ctx"].(*context.Context)

	uriEncoded := GetURI(r.URL.Query().Get("uri"), r.URL.Query().Get("uri_encoded"), true)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	paginatedComments, err := gorm.G[Comment](db).Where("uri_encoded = ?", uriEncoded).Limit(pageSize).Offset((page-1) * pageSize).Order("inserted_at DESC").Find(*ctx)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}



	response := PaginatedResponse{
		Comments:   paginatedComments,
		Page:       page,
		PageSize:   pageSize,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func handlePostComment(w http.ResponseWriter, r *http.Request) {
	db := GlobalCtx["db"].(*gorm.DB)
	ctx := GlobalCtx["ctx"].(*context.Context)

	var req CommentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid JSON"})
		return
	}
	req.URI = r.URL.Query().Get("uri")
	req.URIEncoded =  r.URL.Query().Get("uri_encoded")
	if req.URI == "" && req.URIEncoded == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "uri or uri_encoded is required"})
		return

	}

	if req.Nickname == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Nickname is required"})
		return
	}

	if req.Email == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Email is required"})
		return
	}

	if req.Content == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Content is required"})
		return
	}

	comment := Comment {
		URI: GetURI(req.URI, req.URIEncoded, false),
		URIEncoded: GetURI(req.URI, req.URIEncoded, true),
		Nickname: req.Nickname,
		Email: req.Email,
		Title: req.Title,
		Content: req.Content,
		InsertedAt: time.Now(),
	}
	err := gorm.G[Comment](db).Create(*ctx, &comment)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to create comment"})
		return
	}


	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(comment)
}
