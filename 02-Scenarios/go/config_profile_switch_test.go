package scenarios

import (
	"testing"

	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigProfileSwitching(t *testing.T) {
	testCases := []struct {
		inputProfile string
	}{
		{inputProfile: "standalone"},
		{inputProfile: "devel"},
		{inputProfile: "dev"},
		{inputProfile: "test"},
		{inputProfile: "staging"},
		{inputProfile: "stage"},
		{inputProfile: "production"},
		{inputProfile: "prod"},
	}

	for _, tc := range testCases {
		t.Run("Profile_"+tc.inputProfile, func(t *testing.T) {
			cfg, err := toolbox_config.LoadConfig(tc.inputProfile, nil)
			require.NoError(t, err, "LoadConfig failed for valid profile: %s", tc.inputProfile)
			require.NotNil(t, cfg, "Config should not be nil for profile: %s", tc.inputProfile)
			assert.Equal(t, tc.inputProfile, cfg.Profile, "Config profile mismatch")
		})
	}
}
