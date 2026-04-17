package host

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"infracanvas/pkg/retry"
)

// CloudMetadata represents cloud provider metadata
type CloudMetadata struct {
	Provider         string
	InstanceID       string
	InstanceType     string
	Region           string
	AvailabilityZone string
	Tags             map[string]string
}

// GetCloudMetadata detects and fetches cloud provider metadata
func (d *Discovery) GetCloudMetadata() (*CloudMetadata, error) {
	// Try Azure first — Azure IMDS requires a unique Metadata header and returns
	// structured JSON, so it's the most reliable to distinguish from AWS.
	if metadata, err := getAzureMetadata(); err == nil {
		return metadata, nil
	}

	// Try GCP
	if metadata, err := getGCPMetadata(); err == nil {
		return metadata, nil
	}

	// Try AWS last — its IMDS endpoint (169.254.169.254) can respond on other clouds too
	if metadata, err := getAWSMetadata(); err == nil {
		return metadata, nil
	}

	// Not running on a known cloud provider
	return nil, fmt.Errorf("not running on a known cloud provider")
}

// getAWSMetadata fetches AWS EC2 metadata with retry logic
func getAWSMetadata() (*CloudMetadata, error) {
	baseURL := "http://169.254.169.254/latest/meta-data/"
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	metadata := &CloudMetadata{
		Provider: "aws",
		Tags:     make(map[string]string),
	}

	// Check if we can reach the metadata service with retry
	err := retry.Do(func() error {
		resp, err := client.Get(baseURL)
		if err != nil {
			return fmt.Errorf("AWS metadata service not available: %w", err)
		}
		resp.Body.Close()
		return nil
	}, retry.NetworkConfig())

	if err != nil {
		return nil, err
	}

	// Fetch instance ID
	if instanceID, err := fetchMetadataWithRetry(client, baseURL+"instance-id"); err == nil {
		metadata.InstanceID = instanceID
	}

	// Fetch instance type
	if instanceType, err := fetchMetadataWithRetry(client, baseURL+"instance-type"); err == nil {
		metadata.InstanceType = instanceType
	}

	// Fetch region from availability zone
	if az, err := fetchMetadataWithRetry(client, baseURL+"placement/availability-zone"); err == nil {
		metadata.AvailabilityZone = az
		// Extract region from AZ (e.g., us-east-1a -> us-east-1)
		if len(az) > 0 {
			metadata.Region = az[:len(az)-1]
		}
	}

	// Fetch tags (requires IMDSv2 token, skip if not available)
	if tags, err := fetchAWSTags(client, baseURL); err == nil {
		metadata.Tags = tags
	}

	return metadata, nil
}

// fetchMetadata fetches a single metadata value
func fetchMetadata(client *http.Client, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metadata request failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// fetchMetadataWithRetry fetches a single metadata value with retry logic
func fetchMetadataWithRetry(client *http.Client, url string) (string, error) {
	return retry.DoWithResult(func() (string, error) {
		return fetchMetadata(client, url)
	}, retry.NetworkConfig())
}

// fetchAWSTags fetches AWS instance tags
func fetchAWSTags(client *http.Client, baseURL string) (map[string]string, error) {
	tags := make(map[string]string)

	// Try to get IMDSv2 token
	token, err := getIMDSv2Token(client)
	if err != nil {
		// IMDSv2 not available, skip tags
		return tags, nil
	}

	// Fetch tags with token
	req, err := http.NewRequest("GET", baseURL+"tags/instance", nil)
	if err != nil {
		return tags, err
	}
	req.Header.Set("X-aws-ec2-metadata-token", token)

	resp, err := client.Do(req)
	if err != nil {
		return tags, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return tags, fmt.Errorf("tags request failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return tags, err
	}

	// Parse tag keys
	tagKeys := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, key := range tagKeys {
		if key == "" {
			continue
		}
		// Fetch tag value
		req, err := http.NewRequest("GET", baseURL+"tags/instance/"+key, nil)
		if err != nil {
			continue
		}
		req.Header.Set("X-aws-ec2-metadata-token", token)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		value, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		tags[key] = strings.TrimSpace(string(value))
	}

	return tags, nil
}

// getIMDSv2Token gets an IMDSv2 session token
func getIMDSv2Token(client *http.Client) (string, error) {
	req, err := http.NewRequest("PUT", "http://169.254.169.254/latest/api/token", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "21600")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	token, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(token), nil
}

// getGCPMetadata fetches GCP metadata with retry logic
func getGCPMetadata() (*CloudMetadata, error) {
	baseURL := "http://metadata.google.internal/computeMetadata/v1/"
	
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	metadata := &CloudMetadata{
		Provider: "gcp",
		Tags:     make(map[string]string),
	}

	// Helper function to fetch GCP metadata
	fetchGCPMetadata := func(path string) (string, error) {
		return retry.DoWithResult(func() (string, error) {
			req, err := http.NewRequest("GET", baseURL+path, nil)
			if err != nil {
				return "", err
			}
			req.Header.Set("Metadata-Flavor", "Google")

			resp, err := client.Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("metadata request failed with status %d", resp.StatusCode)
			}

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return "", err
			}

			return strings.TrimSpace(string(data)), nil
		}, retry.NetworkConfig())
	}

	// Check if GCP metadata is available
	if _, err := fetchGCPMetadata("instance/id"); err != nil {
		return nil, fmt.Errorf("GCP metadata service not available: %w", err)
	}

	// Fetch instance ID
	if instanceID, err := fetchGCPMetadata("instance/id"); err == nil {
		metadata.InstanceID = instanceID
	}

	// Fetch instance type (machine type)
	if machineType, err := fetchGCPMetadata("instance/machine-type"); err == nil {
		// Extract just the machine type name from the full path
		parts := strings.Split(machineType, "/")
		if len(parts) > 0 {
			metadata.InstanceType = parts[len(parts)-1]
		}
	}

	// Fetch zone
	if zone, err := fetchGCPMetadata("instance/zone"); err == nil {
		// Extract zone name from full path
		parts := strings.Split(zone, "/")
		if len(parts) > 0 {
			zoneName := parts[len(parts)-1]
			metadata.AvailabilityZone = zoneName
			// Extract region from zone (e.g., us-central1-a -> us-central1)
			if idx := strings.LastIndex(zoneName, "-"); idx > 0 {
				metadata.Region = zoneName[:idx]
			}
		}
	}

	// Fetch tags (labels in GCP)
	if labels, err := fetchGCPMetadata("instance/attributes/"); err == nil {
		// Parse labels
		for _, label := range strings.Split(labels, "\n") {
			if label == "" {
				continue
			}
			if value, err := fetchGCPMetadata("instance/attributes/" + label); err == nil {
				metadata.Tags[label] = value
			}
		}
	}

	return metadata, nil
}

// getAzureMetadata fetches Azure metadata with retry logic
func getAzureMetadata() (*CloudMetadata, error) {
	url := "http://169.254.169.254/metadata/instance?api-version=2021-02-01"
	
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	var azureMetadata struct {
		Compute struct {
			VMID             string `json:"vmId"`
			VMSize           string `json:"vmSize"`
			Location         string `json:"location"`
			Zone             string `json:"zone"`
			Tags             string `json:"tags"`
		} `json:"compute"`
	}

	// Fetch and parse Azure metadata with retry
	err := retry.Do(func() error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Metadata", "true")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("Azure metadata service not available: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Azure metadata request failed with status %d", resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(data, &azureMetadata); err != nil {
			return fmt.Errorf("failed to parse Azure metadata: %w", err)
		}

		return nil
	}, retry.NetworkConfig())

	if err != nil {
		return nil, err
	}

	metadata := &CloudMetadata{
		Provider:         "azure",
		InstanceID:       azureMetadata.Compute.VMID,
		InstanceType:     azureMetadata.Compute.VMSize,
		Region:           azureMetadata.Compute.Location,
		AvailabilityZone: azureMetadata.Compute.Zone,
		Tags:             make(map[string]string),
	}

	// Parse tags (format: "key1:value1;key2:value2")
	if azureMetadata.Compute.Tags != "" {
		tagPairs := strings.Split(azureMetadata.Compute.Tags, ";")
		for _, pair := range tagPairs {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) == 2 {
				metadata.Tags[parts[0]] = parts[1]
			}
		}
	}

	return metadata, nil
}
