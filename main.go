package main

import (
	"context"
	"embed"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hovanhoa/ent-blog/ent/post"
	"html/template"
	"log"
	"net/http"

	"github.com/hovanhoa/ent-blog/ent"

	"github.com/hovanhoa/ent-blog/ent/user"
	_ "github.com/lib/pq"
)

var (
	//go:embed templates/*
	resources embed.FS
	tmpl      = template.Must(template.ParseFS(resources, "templates/*"))
)

type server struct {
	client *ent.Client
}

func newServer(client *ent.Client) *server {
	return &server{client: client}
}

func main() {
	// Instantiate the Ent client.
	client, err := ent.Open("postgres", "host=localhost port=5432 user=postgres dbname=ent_blog password=hovanhoa sslmode=disable")
	if err != nil {
		log.Fatalf("failed connecting to psql: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}

	// If we don't have any posts yet, seed the database.
	if !client.Post.Query().ExistX(ctx) {
		if err := seed(ctx, client); err != nil {
			log.Fatalf("failed seeding the database: %v", err)
		}
	}
	// ... Continue with server start.
	srv := newServer(client)
	r := newRouter(srv)
	log.Fatal(http.ListenAndServe(":8080", r))
}

// index serves the blog home page
func (s *server) index(w http.ResponseWriter, r *http.Request) {
	posts, err := s.client.Post.
		Query().
		WithAuthor().
		Order(ent.Desc(post.FieldCreatedAt)).
		All(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, posts); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// newRouter creates a new router with the blog handlers mounted.
func newRouter(srv *server) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Get("/", srv.index)
	r.Post("/add", srv.add)
	return r
}

// add creates a new blog post.
func (s *server) add(w http.ResponseWriter, r *http.Request) {
	author, err := s.client.User.Query().Only(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.client.Post.Create().
		SetTitle(r.FormValue("title")).
		SetBody(r.FormValue("body")).
		SetAuthor(author).
		Exec(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func seed(ctx context.Context, client *ent.Client) error {
	// Check if the user "hovanhoa" already exists.
	r, err := client.User.Query().
		Where(
			user.Name("hovanhoa"),
		).
		Only(ctx)
	switch {
	// If not, create the user.
	case ent.IsNotFound(err):
		r, err = client.User.Create().
			SetName("hovanhoa").
			SetEmail("r@hello.world").
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed creating user: %v", err)
		}
	case err != nil:
		return fmt.Errorf("failed querying user: %v", err)
	}
	// Finally, create a "Hello, world" blogpost.
	return client.Post.Create().
		SetTitle("Hello, World!").
		SetBody("This is my first post").
		SetAuthor(r).
		Exec(ctx)
}
