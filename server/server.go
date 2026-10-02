package server

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"proj/model"
	"proj/storage"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Server struct {
	store *storage.Storage
	tmpl  *template.Template
}

func New(store *storage.Storage) (*Server, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("шаблоны: %w", err)
	}
	return &Server{store: store, tmpl: tmpl}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	return mux
}

type pageData struct {
	Query string
	User  *model.User
	Error string
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.render(w, http.StatusOK, pageData{})
	case http.MethodPost:
		s.lookup(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
	}
}

// ID приходит из формы методом POST, поэтому в URL его нет.
func (s *Server) lookup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		s.render(w, http.StatusBadRequest, pageData{Error: "некорректная форма"})
		return
	}

	raw := strings.TrimSpace(r.FormValue("id"))
	data := pageData{Query: raw}

	if raw == "" {
		data.Error = "введите ID"
		s.render(w, http.StatusBadRequest, data)
		return
	}

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		data.Error = "ID должен быть положительным числом"
		s.render(w, http.StatusBadRequest, data)
		return
	}

	u, err := s.store.GetUser(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		data.Error = fmt.Sprintf("пользователь с ID %d не найден", id)
		s.render(w, http.StatusNotFound, data)
		return
	}
	if err != nil {
		log.Printf("GetUser(%d): %v", id, err)
		data.Error = "внутренняя ошибка"
		s.render(w, http.StatusInternalServerError, data)
		return
	}

	data.User = &u
	s.render(w, http.StatusOK, data)
}

func (s *Server) render(w http.ResponseWriter, status int, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("шаблон: %v", err)
	}
}
