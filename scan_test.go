package main

import (
	"testing"
)

func TestAWSKeyDetection(t *testing.T) {
	tests := []struct {
		patch  string
		expect bool
	}{
		{"Some random text", false},
		{"AKIAEXAMPLEKEY1234567", true},
		{"Secret Key: AKIA1234AWSSECRETKEY", true},
		{"No AWS key here", false},
	}

	for _, test := range tests {
		result := awsKeyPattern.MatchString(test.patch)
		if result != test.expect {
			t.Errorf("Expected %v, but got %v for patch: %s", test.expect, result, test.patch)
		}
	}
}
