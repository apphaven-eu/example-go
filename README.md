# Go with PostgreSQL on AppHaven

Deploy a Go web application with managed PostgreSQL on [AppHaven](https://apphaven.eu). This example uses `net/http`, `html/template`, and pgx to build a server-rendered todo list without a web framework.

Add, list, and delete tasks. Data persists across app restarts in PostgreSQL.
The repository includes the application, a `Dockerfile`, and an `apphaven.yaml` manifest.

## Stack

- Go 1.25, `net/http` with the Go 1.22 route patterns
- `html/template` for rendering, which escapes values by default
- `github.com/jackc/pgx/v5` (pgxpool), plain SQL, no ORM
- PostgreSQL 17
- Container: `golang:1.25-alpine` build stage, `alpine:3` runtime stage, non-root user

## Run it locally

You need Docker for PostgreSQL and Go 1.25 or newer. Clone this repository first:

```sh
git clone https://github.com/apphaven-eu/example-go.git
cd example-go
```

1. Start PostgreSQL:

   ```
   docker run -d --name todo-pg -p 127.0.0.1:5432:5432 -e POSTGRES_PASSWORD=devpassword postgres:17
   ```

2. Point the app at it:

   ```
   export DATABASE_URL="postgres://postgres:devpassword@127.0.0.1:5432/postgres?sslmode=disable"
   ```

3. Run the app:

   ```
   go run .
   ```

The app listens on http://localhost:8080/. Set `PORT` to use a different port. The `todos` table is
created at startup if it does not exist.

## Deploy Go on AppHaven

1. Fork this repository, or push a copy to a Git host reachable over HTTPS.
2. Open the [AppHaven console](https://console.apphaven.eu/), select a project, and create an app.
3. In **Source**, connect your repository and select the production branch (usually `main`).
4. Click **Deploy** and select that branch. Follow the build logs, then open the deployment URL.

You need an AppHaven account with console access. See the
[getting started guide](https://docs.apphaven.eu/getting-started) for account and repository setup.

`apphaven.yaml` declares the `web` service built from the `Dockerfile` and the managed `db`
service running PostgreSQL 17. `DATABASE_URL` is injected at deploy time from `${service.db.url}`,
so production database credentials stay out of source control.

### Access and shared data

Apps are **private by default**: only members of the AppHaven project can open them.
This example has one shared todo list; it does not separate tasks by user.
For a public demo, a project administrator can select **Public** in the app's **Security**
section and redeploy. Anyone who can reach the app can add and delete tasks, so use demo data.
Production can be public while previews remain private. See [access control](https://docs.apphaven.eu/access).

### Preview a change

Push a new branch and deploy it from the console. AppHaven creates a preview with its own URL,
storage, and database, separate from production. Add a task in the preview, redeploy that branch,
and check that the task is still there before merging the change.

### Verify the deployment

Open the app, add a task, refresh, and delete it. The manifest waits for PostgreSQL to be healthy
before starting the web container. `/healthz` is a process liveness endpoint; it does not query
the database. The container's healthcheck runs internally, so it needs no public-path exemption.

The schema uses `CREATE TABLE IF NOT EXISTS` for the initial table. When extending the app,
use versioned migrations for changes to existing columns and tables.

## AppHaven

AppHaven builds the image from the `Dockerfile` in this repository and runs it. PostgreSQL is a
managed service declared in `apphaven.yaml` rather than something you install and operate.

- Platform: https://apphaven.eu
- Managed PostgreSQL: https://docs.apphaven.eu/services/postgres
- Manifest reference: https://docs.apphaven.eu/reference/manifest

## Related examples

[Spring Boot](https://github.com/apphaven-eu/example-java), [Next.js](https://github.com/apphaven-eu/example-nextjs), [Express](https://github.com/apphaven-eu/example-node), [PHP](https://github.com/apphaven-eu/example-php), [FastAPI](https://github.com/apphaven-eu/example-python).

## License

[MIT](LICENSE).
