# Chirpy

Chirpy is a REST API backend for a simple social media application built with Go. It allows users to register, authenticate, create chirps (posts), and manage their accounts using JWT authentication and refresh tokens. :contentReference[oaicite:0]{index=0}

## Features

- User registration and login
- Password hashing with Argon2
- JWT authentication
- Refresh token authentication
- Create, read, update, and delete chirps
- Filter chirps by author
- Sort chirps by creation time
- User profile updates
- Chirpy Red membership via Polka webhooks
- PostgreSQL database with Goose migrations and SQLC-generated queries :contentReference[oaicite:1]{index=1} :contentReference[oaicite:2]{index=2}

## Tech Stack

- Go
- PostgreSQL
- SQLC
- Goose
- JWT
- Argon2
- UUID

## Project Structure

```
.
├── internal/
│   ├── auth/
│   └── database/
├── sql/
│   ├── queries/
│   └── schema/
├── main.go
└── .env
```

## Environment Variables

Create a `.env` file with:

```env
DB_URL=postgres://username:password@localhost:5432/chirpy?sslmode=disable
SECRET=your_jwt_secret
POLKA_KEY=your_polka_api_key
PLATFORM=dev
```

## API Endpoints

| Method | Endpoint           | Description                                   |
| ------ | ------------------ | --------------------------------------------- |
| POST   | `/api/users`       | Register a user                               |
| PUT    | `/api/users`       | Update authenticated user                     |
| POST   | `/api/login`       | Login and receive access/refresh tokens       |
| POST   | `/api/refresh`     | Refresh access token                          |
| POST   | `/api/revoke`      | Revoke refresh token                          |
| GET    | `/api/chirps`      | List chirps (supports `author_id` and `sort`) |
| GET    | `/api/chirps/{id}` | Get a chirp                                   |
| POST   | `/api/chirps`      | Create a chirp                                |
| DELETE | `/api/chirps/{id}` | Delete your own chirp                         |
| POST   | `/polka/webhooks`  | Upgrade user to Chirpy Red                    |
| GET    | `/api/healthz`     | Health check                                  |


### Authentication

- `POST /api/users`
- `POST /api/login`
- `PUT /api/users`
- `POST /api/refresh`
- `POST /api/revoke`

### Chirps

- `GET /api/chirps`
- `GET /api/chirps/{id}`
- `POST /api/chirps`
- `DELETE /api/chirps/{id}`

### Admin

- `GET /admin/metrics`
- `POST /admin/reset`

### Webhooks

- `POST /polka/webhooks`

## Learning Goals

This project was built to practice:

- Building REST APIs in Go
- Authentication with JWTs
- Refresh token workflows
- Password hashing
- SQLC and Goose
- PostgreSQL schema design
- HTTP middleware
- CRUD operations
- Authorization and protected routes



# Also
- Followed from Boot.dev course
- I do not have a boot.dev membership so I couldn't access the editorials on the site. Hence the code is boud to deviate from the "ideal" sloutions.
