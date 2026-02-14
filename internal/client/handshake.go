package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
)

func (c *Client) tunnelHandshake() error {
	req, _ := http.NewRequest(http.MethodGet, "https://"+c.TunnelAddr+"/", nil)
	req.Host = c.ServerName
	req.Header.Set("Authorization", "Bearer "+c.GenerateAuthToken())

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("tunnelHandshake: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tunnelHandshake: status %d", resp.StatusCode)
	}

	var config struct {
		Flow string `json:"flow"`
		Max  int    `json:"max"`
		TLS  string `json:"tls"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return fmt.Errorf("tunnelHandshake: %w", err)
	}

	c.DataFlow = config.Flow
	c.MaxPoolCapacity = config.Max
	c.TlsCode = config.TLS
	c.PoolType = config.Type

	c.Logger.Info("Loading tunnel config: FLOW=%v|MAX=%v|TLS=%v|TYPE=%v",
		c.DataFlow, c.MaxPoolCapacity, c.TlsCode, c.PoolType)
	return nil
}
