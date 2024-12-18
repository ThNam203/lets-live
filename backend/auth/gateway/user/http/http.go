package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sen1or/lets-live/pkg/discovery"
	"sen1or/lets-live/user/dto"
)

type UserGateway struct {
	registry discovery.Registry
}

type ErrorResponse struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

func NewUserGateway(registry discovery.Registry) *UserGateway {
	return &UserGateway{
		registry: registry,
	}
}

func (g *UserGateway) CreateNewUser(ctx context.Context, userRequestDTO dto.CreateUserRequestDTO) (*dto.CreateUserResponseDTO, *ErrorResponse) {
	addr, err := g.registry.ServiceAddress(ctx, "user")
	if err != nil {
		return nil, &ErrorResponse{
			Message:    err.Error(),
			StatusCode: http.StatusBadGateway,
		}
	}

	url := fmt.Sprintf("http://%s/v1/user", addr)
	payloadBuf := new(bytes.Buffer)
	if err := json.NewEncoder(payloadBuf).Encode(&userRequestDTO); err != nil {
		return nil, &ErrorResponse{
			Message:    fmt.Sprintf("failed to encode user dto body: %s", err),
			StatusCode: http.StatusInternalServerError,
		}
	}

	req, err := http.NewRequest(http.MethodPost, url, payloadBuf)
	if err != nil {
		return nil, &ErrorResponse{
			Message:    fmt.Sprintf("failed to create the request: %s", err),
			StatusCode: http.StatusInternalServerError,
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, &ErrorResponse{
			Message:    fmt.Sprintf("failed to call request: %s", err),
			StatusCode: http.StatusInternalServerError,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		resInfo := ErrorResponse{}
		if err := json.NewDecoder(resp.Body).Decode(&resInfo); err != nil {
			return nil, &ErrorResponse{
				Message:    fmt.Sprintf("failed to decode error response from user service: %s", err),
				StatusCode: http.StatusInternalServerError,
			}
		}

		return nil, &resInfo
	}

	var createdUser dto.CreateUserResponseDTO
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&createdUser); err != nil {
		return nil, &ErrorResponse{
			Message:    fmt.Sprintf("failed to decode resp body: %s", err),
			StatusCode: http.StatusInternalServerError,
		}
	}

	return &createdUser, nil
}
