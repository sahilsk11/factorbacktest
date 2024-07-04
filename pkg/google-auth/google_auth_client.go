package googleauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GetUserDetailsResponse struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"verified_email"`
	PictureUrl    string `json:"picture"`
	FirstName     string `json:"given_name"`
	LastName      string `json:"family_name"`
}

func GetUserDetails(accessToken string) (*GetUserDetailsResponse, error) {
	c := http.DefaultClient
	url := "https://www.googleapis.com/oauth2/v1/userinfo?access_token=" + accessToken
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	var responseJson GetUserDetailsResponse
	responseBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("received status code %d and failed to read body: %w", response.StatusCode, err)
	}
	// fmt.Println(string(responseBytes))
	if response.StatusCode != 200 {
		type errResponse struct {
			Error string `json:"error"`
		}
		errJson := errResponse{}
		err = json.Unmarshal(responseBytes, &errJson)
		if err != nil {
			return nil, fmt.Errorf("received status code %d and failed to read error: %w", response.StatusCode, err)
		}
		return nil, fmt.Errorf("failed with status code %d: %s", response.StatusCode, errJson.Error)
	}

	err = json.Unmarshal(responseBytes, &responseJson)
	if err != nil {
		return nil, err
	}

	return &responseJson, nil
}
