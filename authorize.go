package oauth2Provider

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/satori/go.uuid"
)

type Oauth2Flow string
type ResponseType string
type CodeChallengeMethod string
type AuthorizationRequest struct {
	ClientId            ClientId
	ResponseType        ResponseType
	RedirectUri         string
	Scope               string
	State               string
	codeChallenge       string
	codeChallengeMethod CodeChallengeMethod
	Code                string
}

const (
	RESPONSE_TYPE_CODE  ResponseType = "code"
	RESPONSE_TYPE_TOKEN ResponseType = "token"

	CODE_CHALLENGE_METHOD_PLAIN CodeChallengeMethod = "plain"
	CODE_CHALLENGE_METHOD_S256  CodeChallengeMethod = "S256"
)

func AuthorizationRequestHandler(w http.ResponseWriter, r *http.Request) {

	var authorizationRequest AuthorizationRequest

	//initialize client_id
	if clientId, err := findAndLoadClientSettings(r.URL.Query().Get(PARAM_CLIENT_ID)); err != nil {
		handleOauth2Error(w, err)
		return
	} else {
		authorizationRequest.ClientId = *clientId
	}

	authorizationRequest.ResponseType = ResponseType(r.URL.Query().Get(PARAM_RESPONSE_TYPE))
	authorizationRequest.Scope = r.URL.Query().Get(PARAM_SCOPE)
	authorizationRequest.State = r.URL.Query().Get(PARAM_STATE)

	var err *Oauth2Error
	//Handle authorization code flow request
	switch authorizationRequest.ResponseType {
	case RESPONSE_TYPE_CODE:
		err = handleAuthorizationCodeFlowRequest(w, r, &authorizationRequest)
	case RESPONSE_TYPE_TOKEN:
		err = handleImplicitFlowRequest(w, r, &authorizationRequest)
	default:
		err = NewResponseTypeError()
	}

	if err != nil {
		handleOauth2Error(w, err)
		return
	}
}

/**
 * Even if PKCE (https://tools.ietf.org/html/rfc7636) is not forced, if code_challenge is informed, we will apply it.
 */
func handleAuthorizationCodeFlowRequest(w http.ResponseWriter, r *http.Request, authRequest *AuthorizationRequest) *Oauth2Error {

	//Initialize redirect_uri (required query parameter)
	if redirectUri, err := initRedirectUri(r, authRequest.ClientId.AllowedRedirectUri, false); err != nil {
		return err
	} else {
		authRequest.RedirectUri = redirectUri
	}

	//Get code_challenge, and if client_id settings require use of PKCE, return an error if not respected.
	codeChallenge := r.URL.Query().Get(PARAM_CODE_CHALLENGE)
	if codeChallenge == "" && authRequest.ClientId.ForceUseOfPKCE {
		return NewCodeChallengeError()
	}

	codeChallengeMethod := CodeChallengeMethod(r.URL.Query().Get(PARAM_CODE_CHALLENGE_METHOD))
	//If code_challenge_method is specified, then the value must be plain or S256
	if codeChallengeMethod != "" && codeChallengeMethod != CODE_CHALLENGE_METHOD_PLAIN && codeChallengeMethod != CODE_CHALLENGE_METHOD_S256 {
		return NewCodeChallengeMethodError()
	}

	//If the code_challenge_method is not specified, but there's a code_challenge informed, so we use plain as default
	//For more details, see : https://tools.ietf.org/html/rfc7636#section-4.3
	if codeChallenge != "" && codeChallengeMethod == "" {
		codeChallengeMethod = CODE_CHALLENGE_METHOD_PLAIN
	}

	authRequest.codeChallenge = codeChallenge
	authRequest.codeChallengeMethod = codeChallengeMethod

	//generate code
	authRequest.Code = base64.RawURLEncoding.EncodeToString(uuid.NewV4().Bytes())

	//build redirect uri
	uri, _ := url.Parse(authRequest.RedirectUri)
	query := uri.Query()
	query.Add(PARAM_CODE, authRequest.Code)

	uri.RawQuery = query.Encode()
	http.Redirect(w, r, uri.String(), http.StatusFound)

	//TODO defer code storage

	return nil
}

func handleImplicitFlowRequest(w http.ResponseWriter, r *http.Request, authRequest *AuthorizationRequest) *Oauth2Error {

	//Initialize redirect_uri (optional query parameter)
	if redirectUri, err := initRedirectUri(r, authRequest.ClientId.AllowedRedirectUri, true); err != nil {
		return err
	} else {
		authRequest.RedirectUri = redirectUri
	}

	//TODO generate true jwt
	accessToken := "yoloooo"
	//build redirect uri
	uri, _ := url.Parse(authRequest.RedirectUri)
	query := uri.Query()
	query.Add(PARAM_ACCESS_TOKEN, accessToken)
	query.Add(PARAM_TOKEN_TYPE, string(TOKEN_TYPE_BEARER))
	//TODO add expires_in
	//TODO add scope if different
	if authRequest.State != "" {
		query.Add(PARAM_STATE, authRequest.State)
	}
	uri.RawQuery = query.Encode()
	location := strings.Replace(uri.String(), "?", "#", 1)

	http.Redirect(w, r, location, http.StatusFound)

	return nil
}

/*
 *  On the authorization code flow, the redirect_uri is required : https://tools.ietf.org/html/rfc6749#section-4.1.1
 *  But on implicit flow, it is not mandatory as specified here : https://tools.ietf.org/html/rfc6749#section-4.2.1
 *  In such case we must ensure that the request come's from an allowed client uri https://tools.ietf.org/html/rfc6749#section-3.1.2
 */
func initRedirectUri(r *http.Request, allowedRedirectUris []string, isImplicit bool) (string, *Oauth2Error) {

	//If redirect_uri is not informed and current request is oauth2 implicit flow, then we get it from the settings.
	if redirectUri := r.URL.Query().Get(PARAM_REDIRECT_URI); redirectUri == "" && isImplicit && len(allowedRedirectUris) == 1 {
		return allowedRedirectUris[0], nil
	} else {
		//check that the provided redirect_uri is well informed into the client settings.
		for _, allowedRedirectUri := range allowedRedirectUris {
			if redirectUri == allowedRedirectUri {
				return redirectUri, nil
			}
		}
	}
	//No matching redirect_uri found, return an error.
	return "", NewRedirectUriError(isImplicit)
}