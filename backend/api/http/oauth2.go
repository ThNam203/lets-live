package http

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sen1or/lets-live/api/domains"
	"time"

	"github.com/gofrs/uuid/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

type googleOAuthUser struct {
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
}

const oauthGoogleUrlAPI = "https://www.googleapis.com/oauth2/v2/userinfo?access_token="

var googleOauthConfig = &oauth2.Config{
	RedirectURL:  "http://localhost:8000/v1/auth/google/callback",
	ClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
	ClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
	Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
	Endpoint:     google.Endpoint,
}

func (a *APIServer) OAuthGoogleLogin(w http.ResponseWriter, r *http.Request) {
	oauthState, err := generateOAuthCookieState(w)
	if err != nil {
		a.errorResponse(w, http.StatusInternalServerError, err)
		return
	}

	u := googleOauthConfig.AuthCodeURL(oauthState)
	http.Redirect(w, r, u, http.StatusTemporaryRedirect)
}

func generateOAuthCookieState(w http.ResponseWriter) (string, error) {
	var expiration = time.Now().Add(1 * time.Hour)

	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	state := base64.URLEncoding.EncodeToString(b)
	cookie := &http.Cookie{Name: "oauthstate", Value: state, Expires: expiration}
	http.SetCookie(w, cookie)

	return state, nil
}

func (a *APIServer) OAuthGoogleCallBack(w http.ResponseWriter, r *http.Request) {
	oauthState, _ := r.Cookie("oauthstate")
	clientAddr := os.Getenv("CLIENT_URL")
	urlDirectOnFail := clientAddr + "/auth/login"

	if r.FormValue("state") != oauthState.Value {
		a.setError(w, fmt.Errorf("invalid state, csrf attack?"))
		http.Redirect(w, r, urlDirectOnFail, http.StatusTemporaryRedirect)
		return
	}

	data, err := getUserDataFromGoogle(r.FormValue("code"))
	if err != nil {
		a.setError(w, fmt.Errorf("can't get user data"))
		http.Redirect(w, r, urlDirectOnFail, http.StatusTemporaryRedirect)
		return
	}

	var returnedOAuthUser googleOAuthUser
	if err := json.Unmarshal(data, &returnedOAuthUser); err != nil {
		a.setError(w, fmt.Errorf("user data format not valid"))
		http.Redirect(w, r, urlDirectOnFail, http.StatusTemporaryRedirect)
		return
	}

	// TODO: MORE PROPER WAY TO HANDLE USERNAME
	userId, _ := uuid.NewGen().NewV4()
	usernameId, _ := uuid.NewGen().NewV4()
	username := "ll-" + usernameId.String()[:5]

	newOAuthUserRecord := &domains.User{
		ID:         userId,
		Username:   username,
		Email:      returnedOAuthUser.Email,
		IsVerified: returnedOAuthUser.VerifiedEmail,
	}

	// The final UserId that will be used to generate token pair
	var finalUserId uuid.UUID

	// Check if the user had already singed up before
	existedRecord, err := a.userRepo.GetByEmail(returnedOAuthUser.Email)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		err = a.userRepo.Create(*newOAuthUserRecord)

		if err != nil {
			a.setError(w, fmt.Errorf("error while saving user"))
			http.Redirect(w, r, urlDirectOnFail, http.StatusTemporaryRedirect)
			return
		}

		finalUserId = newOAuthUserRecord.ID
	} else {
		finalUserId = existedRecord.ID
		a.refreshTokenRepo.RevokeAll(finalUserId)
	}

	refreshToken, accessToken, err := a.refreshTokenRepo.GenerateTokenPair(finalUserId)

	if err != nil {
		a.setError(w, err)
		http.Redirect(w, r, urlDirectOnFail, http.StatusTemporaryRedirect)
		return
	}

	a.setTokens(w, refreshToken, accessToken)
	http.Redirect(w, r, clientAddr, http.StatusTemporaryRedirect)
}

func getUserDataFromGoogle(code string) ([]byte, error) {
	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("code exchange not valid: %s", err.Error())
	}

	reponse, err := http.Get(oauthGoogleUrlAPI + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}

	defer reponse.Body.Close()
	userData, err := io.ReadAll(reponse.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading user info: %s", err.Error())
	}

	return userData, nil
}
