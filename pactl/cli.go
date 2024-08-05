package pactl

import "os/exec"

// GetSinkInputs retrieves the list of sink input IDs from PulseAudio.
func GetSinkInputs() ([]*SinkInput, error) {
	cmd := exec.Command("pactl", "list", "sink-inputs")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	pactlOutput, err := parsePactlOutput(string(output))
	if err != nil {
		return nil, err
	}

	return pactlOutput, nil
}
