package main

import (
	"context"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type todo struct {
	ID    int64
	Title string
}

type server struct {
	pool *pgxpool.Pool
	tmpl *template.Template
}

const schema = `
CREATE TABLE IF NOT EXISTS todos (
  id         BIGSERIAL PRIMARY KEY,
  title      TEXT        NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Go Todo | AppHaven</title>
<style>
  * { box-sizing: border-box; }
  input[type=text] { min-width: 0; }
  li span { min-width: 0; overflow-wrap: anywhere; }
  li form { flex-shrink: 0; }
  footer { margin-top: 2rem; padding-top: 1rem; border-top: 1px solid #8884; font-size: .875rem; }
  a { color: #2457bd; }
  :focus-visible { outline: 2px solid #2457bd; outline-offset: 3px; }

  :root { color-scheme: light; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    line-height: 1.5; margin: 0; padding: 3rem 1rem;
  }
  main { max-width: 640px; margin: 0 auto; }
  h1 { font-size: 1.5rem; margin: 0 0 1.5rem; }
  form.add { display: flex; gap: .5rem; margin-bottom: 1.5rem; }
  input[type=text] {
    flex: 1; padding: .55rem .7rem; font: inherit;
    border: 1px solid #8888; border-radius: 6px; background: transparent; color: inherit;
  }
  button {
    padding: .55rem .9rem; font: inherit; cursor: pointer;
    border: 1px solid #8888; border-radius: 6px; background: transparent; color: inherit;
  }
  button:hover { border-color: currentColor; }
  ul { list-style: none; margin: 0; padding: 0; }
  li {
    display: flex; align-items: center; gap: 1rem;
    padding: .7rem 0; border-top: 1px solid #8883;
  }
  li span { flex: 1; overflow-wrap: anywhere; }
  li form { margin: 0; }
  li button { padding: .25rem .6rem; font-size: .85rem; }
  p.empty { color: #8a8a8a; border-top: 1px solid #8883; padding-top: .9rem; }
</style>
</head>
<body>
<main>
  <h1>Go Todo</h1><p>A shared task list, built with Go and PostgreSQL.</p>
  <form class="add" method="post" action="/add">
    <input type="text" name="title" aria-label="New task" placeholder="What needs doing?" maxlength="200" required autofocus>
    <button type="submit">Add</button>
  </form>
  {{if .}}
  <ul>
    {{range .}}
    <li>
      <span>{{.Title}}</span>
      <form method="post" action="/delete">
        <input type="hidden" name="id" value="{{.ID}}">
        <button type="submit">Delete</button>
      </form>
    </li>
    {{end}}
  </ul>
  {{else}}
  <p class="empty">No items yet.</p>
  {{end}}
<footer><p>Deploy your own on <a href="https://apphaven.eu">AppHaven</a> · <a href="https://github.com/apphaven-eu/example-go">Source code</a> · <a href="https://docs.apphaven.eu/getting-started">Deployment guide</a></p></footer></main>
</body>
</html>
`))

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if err := migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	s := &server{pool: pool, tmpl: page}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /{$}", s.index)
	mux.HandleFunc("POST /add", s.add)
	mux.HandleFunc("POST /delete", s.delete)

	srv := &http.Server{
		Addr:              "0.0.0.0:" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve: %v", err)
		}
	}()

	<-ctx.Done()
	stop()
	log.Print("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

// migrate retries because the database may not be accepting connections yet when the app starts.
func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		if _, err = pool.Exec(ctx, schema); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return err
}

func (s *server) index(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(),
		"SELECT id, title FROM todos ORDER BY created_at DESC, id DESC")
	if err != nil {
		httpError(w, err)
		return
	}
	defer rows.Close()

	var todos []todo
	for rows.Next() {
		var t todo
		if err := rows.Scan(&t.ID, &t.Title); err != nil {
			httpError(w, err)
			return
		}
		todos = append(todos, t)
	}
	if err := rows.Err(); err != nil {
		httpError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, todos); err != nil {
		log.Printf("render: %v", err)
	}
}

func (s *server) add(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.FormValue("title"))
	if title != "" {
		if chars := []rune(title); len(chars) > 200 {
			title = string(chars[:200])
		}
		if _, err := s.pool.Exec(r.Context(),
			"INSERT INTO todos (title) VALUES ($1)", title); err != nil {
			httpError(w, err)
			return
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if _, err := s.pool.Exec(r.Context(), "DELETE FROM todos WHERE id = $1", id); err != nil {
		httpError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func httpError(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
