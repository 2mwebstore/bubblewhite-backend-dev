package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// FacebookUser is the verified identity extracted from a Facebook access
// token — only what CustomerService actually needs.
type FacebookUser struct {
	ID    string // Facebook's stable user ID — what FacebookID links against
	Email string // may be empty: Facebook lets a user decline to share email, or sign up with a phone number instead
	Name  string
}

type FacebookOAuthService struct {
	appID     string
	appSecret string
}

func NewFacebookOAuthService(appID, appSecret string) *FacebookOAuthService {
	return &FacebookOAuthService{appID: appID, appSecret: appSecret}
}

var ErrFacebookNotConfigured = errors.New("facebook sign-in is not configured")
var ErrInvalidFacebookToken = errors.New("invalid facebook token")

// debugTokenResponse mirrors the relevant part of
// GET /debug_token?input_token=...&access_token={app-id}|{app-secret}.
type debugTokenResponse struct {
	Data struct {
		AppID   string `json:"app_id"`
		IsValid bool   `json:"is_valid"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"data"`
}

type facebookMeResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// VerifyAccessToken confirms a Facebook access token is genuinely valid
// AND was issued to this specific app (not a token a malicious client
// obtained from an unrelated Facebook app and is replaying against this
// backend), then fetches the profile it belongs to.
//
// Two separate Graph API calls, matching Facebook's own documented
// pattern for server-side verification of a client-obtained token:
//  1. debug_token — using this app's own app_id|app_secret as the
//     "inspector" token — confirms the token is valid and its app_id
//     matches ours.
//  2. /me — fetches the actual profile fields once the token is
//     confirmed to belong to this app.
func (f *FacebookOAuthService) VerifyAccessToken(accessToken string) (*FacebookUser, error) {
	if f.appID == "" || f.appSecret == "" {
		return nil, ErrFacebookNotConfigured
	}
	if accessToken == "" {
		return nil, ErrInvalidFacebookToken
	}

	if err := f.verifyTokenBelongsToThisApp(accessToken); err != nil {
		return nil, err
	}

	meURL := "https://graph.facebook.com/me?" + url.Values{
		"fields":       {"id,name,email"},
		"access_token": {accessToken},
	}.Encode()

	resp, err := http.Get(meURL)
	if err != nil {
		return nil, fmt.Errorf("calling facebook /me: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading facebook /me response: %w", err)
	}

	var me facebookMeResponse
	if err := json.Unmarshal(body, &me); err != nil {
		return nil, fmt.Errorf("parsing facebook /me response: %w", err)
	}
	if me.ID == "" {
		return nil, fmt.Errorf("%w: facebook returned no user id", ErrInvalidFacebookToken)
	}

	return &FacebookUser{ID: me.ID, Email: me.Email, Name: me.Name}, nil
}

func (f *FacebookOAuthService) verifyTokenBelongsToThisApp(accessToken string) error {
	appAccessToken := f.appID + "|" + f.appSecret
	debugURL := "https://graph.facebook.com/debug_token?" + url.Values{
		"input_token":  {accessToken},
		"access_token": {appAccessToken},
	}.Encode()

	resp, err := http.Get(debugURL)
	if err != nil {
		return fmt.Errorf("calling facebook debug_token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading facebook debug_token response: %w", err)
	}

	var parsed debugTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("parsing facebook debug_token response: %w", err)
	}

	if !parsed.Data.IsValid {
		msg := "token is not valid"
		if parsed.Data.Error != nil && parsed.Data.Error.Message != "" {
			msg = parsed.Data.Error.Message
		}
		return fmt.Errorf("%w: %s", ErrInvalidFacebookToken, msg)
	}
	if parsed.Data.AppID != f.appID {
		return fmt.Errorf("%w: token was not issued for this app", ErrInvalidFacebookToken)
	}
	return nil
}
