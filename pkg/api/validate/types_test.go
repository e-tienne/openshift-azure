package validate

import (
	"net"
	"testing"
)

func TestValidateImageVersion(t *testing.T) {
	invalidVersions := []string{
		".1.1",
		"1.1",
		"1.1.123456789",
		"1.12345.1",
		"1234.1.1",
		"1.1",
		"1",
	}
	for _, invalidVersion := range invalidVersions {
		if rxImageVersion.MatchString(invalidVersion) {
			t.Errorf("invalid Version passed test: %s", invalidVersion)
		}
	}
	validVersions := []string{
		"123.1.12345678",
		"123.123.12345678",
		"123.1234.12345678",
	}
	for _, validVersion := range validVersions {
		if !rxImageVersion.MatchString(validVersion) {
			t.Errorf("valid Version failed to test: %s", validVersion)
		}
	}
}

func TestValidateClusterVersion(t *testing.T) {
	invalidVersions := []string{
		".1.1",
		"1.1",
		"1.1.1",
		"v1",
		"v1.0.0",
	}
	for _, invalidVersion := range invalidVersions {
		if rxClusterVersion.MatchString(invalidVersion) {
			t.Errorf("invalid Version passed test: %s", invalidVersion)
		}
	}
	validVersions := []string{
		"v1.0",
		"v123.123456789",
	}
	for _, validVersion := range validVersions {
		if !rxClusterVersion.MatchString(validVersion) {
			t.Errorf("valid Version failed to test: %s", validVersion)
		}
	}
}

func TestIsValidCloudAppHostname(t *testing.T) {
	invalidFqdns := []string{
		"invalid.random.domain",
		"too.long.domain.cloudapp.azure.com",
		"invalid#characters#domain.westus2.cloudapp.azure.com",
		"wronglocation.eastus.cloudapp.azure.com",
		"123.eastus.cloudapp.azure.com",
		"-abc.eastus.cloudapp.azure.com",
		"abcdefghijklmnopqrstuvwxzyabcdefghijklmnopqrstuvwxzyabcdefghijkl.eastus.cloudapp.azure.com",
		"a/b/c.eastus.cloudapp.azure.com",
		".eastus.cloudapp.azure.com",
		"Thisisatest.eastus.cloudapp.azure.com",
	}
	for _, invalidFqdn := range invalidFqdns {
		if isValidCloudAppHostname(invalidFqdn, "westus2") {
			t.Errorf("invalid FQDN passed test: %s", invalidFqdn)
		}
	}
	validFqdns := []string{
		"example.westus2.cloudapp.azure.com",
		"test-dashes.westus2.cloudapp.azure.com",
		"test123.westus2.cloudapp.azure.com",
		"test-123.westus2.cloudapp.azure.com",
	}
	for _, validFqdn := range validFqdns {
		if !isValidCloudAppHostname(validFqdn, "westus2") {
			t.Errorf("Valid FQDN failed to pass test: %s", validFqdn)
		}
	}
}

func TestIsValidIPV4CIDR(t *testing.T) {
	for _, test := range []struct {
		cidr  string
		valid bool
	}{
		{
			cidr: "",
		},
		{
			cidr: "foo",
		},
		{
			cidr: "::/0",
		},
		{
			cidr: "192.168.0.1/24",
		},
		{
			cidr:  "192.168.0.0/24",
			valid: true,
		},
	} {
		valid := isValidIPV4CIDR(test.cidr)
		if valid != test.valid {
			t.Errorf("%s: unexpected result %v", test.cidr, valid)
		}
	}
}

func TestVnetContainsSubnet(t *testing.T) {
	for i, test := range []struct {
		vnetCidr   string
		subnetCidr string
		valid      bool
	}{
		{
			vnetCidr:   "10.0.0.0/16",
			subnetCidr: "192.168.0.0/16",
		},
		{
			vnetCidr:   "10.0.0.0/16",
			subnetCidr: "10.0.0.0/8",
		},
		{
			vnetCidr:   "10.0.0.0/16",
			subnetCidr: "10.0.128.0/15",
		},
		{
			vnetCidr:   "10.0.0.0/8",
			subnetCidr: "10.0.0.0/16",
			valid:      true,
		},
		{
			vnetCidr:   "10.0.0.0/8",
			subnetCidr: "10.0.0.0/8",
			valid:      true,
		},
	} {
		_, vnet, err := net.ParseCIDR(test.vnetCidr)
		if err != nil {
			t.Fatal(err)
		}

		_, subnet, err := net.ParseCIDR(test.subnetCidr)
		if err != nil {
			t.Fatal(err)
		}

		valid := vnetContainsSubnet(vnet, subnet)
		if valid != test.valid {
			t.Errorf("%d: unexpected result %v", i, valid)
		}
	}
}

func TestIsValidAgentPoolHostname(t *testing.T) {
	for _, tt := range []struct {
		hostname string
		valid    bool
	}{
		{
			hostname: "bad",
		},
		{
			hostname: "master-000000",
			valid:    true,
		},
		{
			hostname: "master-00000a",
			valid:    true,
		},
		{
			hostname: "master-00000A",
			valid:    true,
		},
		{
			hostname: "mycompute-000000",
		},
		{
			hostname: "master-bad",
		},
		{
			hostname: "master-inval!",
		},
		{
			hostname: "mycompute-1234567890-000000",
			valid:    true,
		},
		{
			hostname: "mycompute-1234567890-00000z",
			valid:    true,
		},
		{
			hostname: "mycompute-1234567890-00000Z",
			valid:    true,
		},
		{
			hostname: "mycompute-1234-00000Z",
		},
		{
			hostname: "mycompute-1234567890-bad",
		},
		{
			hostname: "mycompute-1234567890-inval!",
		},
		{
			hostname: "master-1234567890-000000",
		},
		{
			hostname: "bad-bad-bad-bad",
		},
	} {
		valid := IsValidAgentPoolHostname(tt.hostname)
		if valid != tt.valid {
			t.Errorf("%s: wanted valid %v, got %v", tt.hostname, tt.valid, valid)
		}
	}
}

func TestIsValidBlobContainerName(t *testing.T) {
	for _, tt := range []struct {
		name  string
		valid bool
	}{
		{
			name: "12",
		},
		{
			name:  "123",
			valid: true,
		},
		{
			name:  "abc",
			valid: true,
		},
		{
			name:  "abc-123",
			valid: true,
		},
		{
			name:  "123456789012345678901234567890123456789012345678901234567890123",
			valid: true,
		},
		{
			name: "1234567890123456789012345678901234567890123456789012345678901234",
		},
		{
			name: "bad!",
		},
		{
			name: "Bad",
		},
		{
			name: "-bad",
		},
		{
			name: "bad-",
		},
		{
			name: "bad--bad",
		},
	} {
		valid := IsValidBlobContainerName(tt.name)
		if valid != tt.valid {
			t.Errorf("%s: wanted valid %v, got %v", tt.name, tt.valid, valid)
		}
	}
}
