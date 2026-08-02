package main

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	secret         string
	polkaKey       string
}

type UserResponse struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
}

type ChirpResponse struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) error {
	response, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
	return nil
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	respondWithJSON(w, code, map[string]string{"error": msg})
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))

}

func (cfg *apiConfig) hitsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, `
<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileserverHits.Load())
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	platform := os.Getenv("PLATFORM")
	if platform != "dev" {
		respondWithError(w, http.StatusForbidden, "not a dev")
		return
	}
	err := cfg.db.RemoveUser(r.Context())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "couldn't remove the table")
		return
	}
	cfg.fileserverHits.Store(0)
}

func (cfg *apiConfig) userHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	type requestBody struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	var param requestBody
	err := json.NewDecoder(r.Body).Decode(&param)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Cannot read the table")
		return
	}

	hashed_pass, err := auth.HashPassword(param.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't hash password")
		return
	}

	params := database.CreateUserParams{
		HashedPassword: hashed_pass,
		Email:          param.Email,
	}

	user, err := cfg.db.CreateUser(r.Context(), params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "couldn't create user")
		return
	}
	respondWithJSON(w, http.StatusCreated, UserResponse{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	})

}

func (cfg *apiConfig) userUpdateHandler(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid header")
		return
	}

	userID, err := auth.ValidateJWT(bearerToken, cfg.secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	type requestBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	req := requestBody{}
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "error reading")
		return
	}

	hasedPass, err := auth.HashPassword(req.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "cannot hash")
		return
	}

	param := database.UpdateUserByIDParams{
		Email:          req.Email,
		HashedPassword: hasedPass,
		ID:             userID,
	}

	user, err := cfg.db.UpdateUserByID(r.Context(), param)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "cannot find user with token")
		return
	}

	if err = respondWithJSON(w, http.StatusOK, UserResponse{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

func (cfg *apiConfig) upgradeUserHandler(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	apiKey, err := auth.GetAPIKey(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if cfg.polkaKey != apiKey {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	type datatype struct {
		UserID uuid.UUID `json:"user_id"`
	}

	type requestBody struct {
		Event string   `json:"event"`
		Data  datatype `json:"data"`
	}

	req := requestBody{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid json")
		return
	}

	if req.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := cfg.db.UpgradeUser(r.Context(), req.Data.UserID); err != nil {
		respondWithError(w, http.StatusNotFound, "cannot upgader user")
		return
	}

	w.WriteHeader(http.StatusNoContent)

}

func (cfg *apiConfig) chirpHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	type requestBody struct {
		Body string `json:"body"`
	}

	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "not a token")
		return
	}

	userID, err := auth.ValidateJWT(bearerToken, cfg.secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	param := requestBody{}
	err = json.NewDecoder(r.Body).Decode(&param)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if len(param.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "chirp is too long")
		return
	}

	words := strings.Split(param.Body, " ")

	for i, str := range words {
		lower_word := strings.ToLower(str)
		if lower_word == "kerfuffle" || lower_word == "sharbert" || lower_word == "fornax" {
			words[i] = "****"
			continue
		}
	}

	param.Body = strings.Join(words, " ")
	params := database.CreateChirpParams{
		Body:   param.Body,
		UserID: userID,
	}

	chirp, err := cfg.db.CreateChirp(r.Context(), params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "couldn't create chirp")
		return
	}
	if err = respondWithJSON(w, http.StatusCreated, ChirpResponse{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

func (cfg *apiConfig) chirpGetHandler(w http.ResponseWriter, r *http.Request) {

	authorIDStr := r.URL.Query().Get("author_id")
	authorID, err := uuid.Parse(authorIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Erro converting str to uuid")
	}

	chirps, err := cfg.db.ReadAllChirps(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't read chirps")
		return
	}

	sortOrder := r.URL.Query().Get("sort")

	if sortOrder == "desc" {
		sort.Slice(chirps, func(i, j int) bool {
			return chirps[i].CreatedAt.After(chirps[j].CreatedAt)
		})
	} else {
		sort.Slice(chirps, func(i, j int) bool {
			return chirps[i].CreatedAt.Before(chirps[j].CreatedAt)
		})
	}

	responses := make([]ChirpResponse, 0, len(chirps))

	if authorIDStr != "" {
		for _, chirp := range chirps {
			if chirp.UserID == authorID {
				responses = append(responses, ChirpResponse{
					ID:        chirp.ID,
					CreatedAt: chirp.CreatedAt,
					UpdatedAt: chirp.UpdatedAt,
					Body:      chirp.Body,
					UserID:    chirp.UserID,
				})
			}
		}
	} else {
		for _, chirp := range chirps {
			responses = append(responses, ChirpResponse{
				ID:        chirp.ID,
				CreatedAt: chirp.CreatedAt,
				UpdatedAt: chirp.UpdatedAt,
				Body:      chirp.Body,
				UserID:    chirp.UserID,
			})
		}
	}

	if err := respondWithJSON(w, http.StatusOK, responses); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (cfg *apiConfig) getchirpByIDHandler(w http.ResponseWriter, r *http.Request) {

	pathvalue := r.PathValue("chirpID")
	id, err := uuid.Parse(pathvalue)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid chirp id")
		return
	}

	chirp, err := cfg.db.GetChirp(r.Context(), id)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "id does not exists")
		return
	}

	respondWithJSON(w, http.StatusOK, ChirpResponse{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
}

func (cfg *apiConfig) deleteChirpByIDHandler(w http.ResponseWriter, r *http.Request) {
	pathvalue := r.PathValue("chirpID")
	id, err := uuid.Parse(pathvalue)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid chirp id")
		return
	}

	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid header")
		return
	}

	userID, err := auth.ValidateJWT(bearerToken, cfg.secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	chirp, err := cfg.db.GetChirp(r.Context(), id)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "chirp not found")
		return
	}

	if chirp.UserID != userID {
		respondWithError(w, http.StatusForbidden, "not your chirp")
		return
	}

	err = cfg.db.RemoveChirpByID(r.Context(), id)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "id does not exists")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	type requestBody struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	param := requestBody{}
	if err := json.NewDecoder(r.Body).Decode(&param); err != nil {
		respondWithError(w, http.StatusBadRequest, "couldn't read json")
		return
	}

	user, err := cfg.db.GetUser(r.Context(), param.Email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	match, err := auth.CheckPasswordHash(param.Password, user.HashedPassword)
	if err != nil || !match {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	tok, err := auth.MakeJWT(user.ID, cfg.secret, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error creating token")
		return
	}

	refTok := auth.MakeRefreshToken()
	refparams := database.CreateRefreshTokenParams{
		Token:     refTok,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(60 * 24 * time.Hour),
	}

	_, err = cfg.db.CreateRefreshToken(r.Context(), refparams)
	if err != nil {
		fmt.Println(err)
		respondWithError(w, http.StatusBadRequest, "cannot create token")
		return
	}

	if err = respondWithJSON(w, http.StatusOK, UserResponse{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        tok,
		RefreshToken: refTok,
		IsChirpyRed:  user.IsChirpyRed,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

func (cfg *apiConfig) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid header")
		return
	}

	type tokenResponse struct {
		Token string `json:"token"`
	}

	refreshTokenParams, err := cfg.db.GetRefreshToken(r.Context(), refreshToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	if refreshTokenParams.ExpiresAt.Before(time.Now()) || refreshTokenParams.RevokedAt.Valid {
		respondWithError(w, http.StatusUnauthorized, "token expired")
		return
	}

	user, err := cfg.db.GetUserByID(r.Context(), refreshTokenParams.UserID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "cannot find user with token")
		return
	}
	accessToken, err := auth.MakeJWT(user.ID, cfg.secret, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't create access token")
		return
	}

	if err = respondWithJSON(w, http.StatusOK, tokenResponse{
		Token: accessToken,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

func (cfg *apiConfig) revokeTokenHandler(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid header")
		return
	}

	type tokenResponse struct {
	}

	err = cfg.db.RevokeRefreshToken(r.Context(), refreshToken)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error revoking token")
		return
	}
	w.WriteHeader(http.StatusNoContent)

}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Print("Error loading env")
	}

	const root = "."
	const port = "8080"

	dbURL := os.Getenv("DB_URL")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Print(err)
	}

	apiCfg := apiConfig{}
	apiCfg.db = database.New(db)
	apiCfg.secret = os.Getenv("SECRET")
	apiCfg.polkaKey = os.Getenv("POLKA_KEY")

	mux := http.NewServeMux()
	server := &http.Server{
		Handler: mux,
		Addr:    ":" + port,
	}

	handler := http.StripPrefix("/app", http.FileServer(http.Dir(root)))

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))
	mux.HandleFunc("GET /api/healthz", readinessHandler)
	mux.HandleFunc("GET /admin/metrics", apiCfg.hitsHandler)
	mux.HandleFunc("GET /api/chirps", apiCfg.chirpGetHandler)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getchirpByIDHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)
	mux.HandleFunc("POST /api/chirps", apiCfg.chirpHandler)
	mux.HandleFunc("POST /api/users", apiCfg.userHandler)
	mux.HandleFunc("POST /api/login", apiCfg.loginHandler)
	mux.HandleFunc("POST /api/refresh", apiCfg.refreshTokenHandler)
	mux.HandleFunc("POST /api/revoke", apiCfg.revokeTokenHandler)
	mux.HandleFunc("POST /polka/webhooks", apiCfg.upgradeUserHandler)
	mux.HandleFunc("PUT /api/users", apiCfg.userUpdateHandler)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.deleteChirpByIDHandler)

	if err := server.ListenAndServe(); err != nil {
		fmt.Print(err)
	}

}
