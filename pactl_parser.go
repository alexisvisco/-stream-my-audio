package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

type SinkInput struct {
	ID                int
	Driver            string
	OwnerModule       string
	Client            int
	Sink              int
	SampleSpec        string
	ChannelMap        string
	Format            string
	Corked            bool
	Mute              bool
	Volume            map[string]float64
	Balance           float64
	BufferLatency     int
	SinkLatency       int
	ResampleMethod    string
	Properties        Properties
	UnknownProperties map[string]string
}

type Properties struct {
	ClientAPI                   string
	PulseServerType             string
	ApplicationName             string
	ApplicationProcessID        string
	ApplicationProcessUser      string
	ApplicationProcessHost      string
	ApplicationProcessBinary    string
	ApplicationLanguage         string
	WindowX11Display            string
	ApplicationProcessMachineID string
	MediaName                   string
	NodeRate                    string
	NodeLatency                 string
	StreamIsLive                string
	NodeName                    string
	NodeWantDriver              string
	NodeAutoconnect             string
	MediaClass                  string
	PortGroup                   string
	AdaptFollowerSpaNode        string
	ObjectRegister              string
	FactoryID                   string
	ClockQuantumLimit           string
	NodeLoopName                string
	LibraryName                 string
	ClientID                    string
	ObjectID                    string
	ObjectSerial                string
	PulseAttrMaxlength          string
	PulseAttrTlength            string
	PulseAttrPrebuf             string
	PulseAttrMinreq             string
	NodeDriverID                string
	ModuleStreamRestoreID       string
}

func parsePactlOutput(output string) ([]*SinkInput, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var sinkInputs []*SinkInput
	var currentSinkInput *SinkInput
	var currentSection string

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "Sink Input #") {
			if currentSinkInput != nil {
				sinkInputs = append(sinkInputs, currentSinkInput)
			}
			id, err := strconv.Atoi(strings.TrimPrefix(line, "Sink Input #"))
			if err != nil {
				return nil, fmt.Errorf("error parsing Sink Input ID on line %d: %v", lineNumber, err)
			}
			currentSinkInput = &SinkInput{
				ID:                id,
				Volume:            make(map[string]float64),
				UnknownProperties: make(map[string]string),
			}
			currentSection = ""
		} else if currentSinkInput != nil {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "Driver":
					currentSinkInput.Driver = value
				case "Owner Module":
					currentSinkInput.OwnerModule = value
				case "Client":
					currentSinkInput.Client, _ = strconv.Atoi(value)
				case "Sink":
					currentSinkInput.Sink, _ = strconv.Atoi(value)
				case "Sample Specification":
					currentSinkInput.SampleSpec = value
				case "Channel Map":
					currentSinkInput.ChannelMap = value
				case "Format":
					currentSinkInput.Format = value
				case "Corked":
					currentSinkInput.Corked = value == "yes"
				case "Mute":
					currentSinkInput.Mute = value == "yes"
				case "Volume":
					currentSection = "volume"
				case "Buffer Latency":
					currentSinkInput.BufferLatency, _ = strconv.Atoi(strings.Split(value, " ")[0])
				case "Sink Latency":
					currentSinkInput.SinkLatency, _ = strconv.Atoi(strings.Split(value, " ")[0])
				case "Resample method":
					currentSinkInput.ResampleMethod = value
				case "Properties":
					currentSection = "properties"
				}
			} else if currentSection == "volume" {
				if strings.HasPrefix(line, "balance") {
					currentSinkInput.Balance, _ = strconv.ParseFloat(strings.Split(line, " ")[1], 64)
				} else {
					parts := strings.Split(line, ":")
					channel := strings.TrimSpace(parts[0])
					volumeParts := strings.Split(parts[1], "/")
					volume, _ := strconv.ParseFloat(strings.TrimSpace(volumeParts[1]), 64)
					currentSinkInput.Volume[channel] = volume
				}
			} else if currentSection == "properties" {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
					setProperty(&currentSinkInput.Properties, key, value, currentSinkInput.UnknownProperties)
				}
			}
		}
	}

	// Add the last sink input
	if currentSinkInput != nil {
		sinkInputs = append(sinkInputs, currentSinkInput)
	}

	return sinkInputs, nil
}

func setProperty(props *Properties, key, value string, unknownProps map[string]string) {
	switch key {
	case "client.api":
		props.ClientAPI = value
	case "pulse.server.type":
		props.PulseServerType = value
	case "application.name":
		props.ApplicationName = value
	case "application.process.id":
		props.ApplicationProcessID = value
	case "application.process.user":
		props.ApplicationProcessUser = value
	case "application.process.host":
		props.ApplicationProcessHost = value
	case "application.process.binary":
		props.ApplicationProcessBinary = value
	case "application.language":
		props.ApplicationLanguage = value
	case "window.x11.display":
		props.WindowX11Display = value
	case "application.process.machine_id":
		props.ApplicationProcessMachineID = value
	case "media.name":
		props.MediaName = value
	case "node.rate":
		props.NodeRate = value
	case "node.latency":
		props.NodeLatency = value
	case "stream.is-live":
		props.StreamIsLive = value
	case "node.name":
		props.NodeName = value
	case "node.want-driver":
		props.NodeWantDriver = value
	case "node.autoconnect":
		props.NodeAutoconnect = value
	case "media.class":
		props.MediaClass = value
	case "port.group":
		props.PortGroup = value
	case "adapt.follower.spa-node":
		props.AdaptFollowerSpaNode = value
	case "object.register":
		props.ObjectRegister = value
	case "factory.id":
		props.FactoryID = value
	case "clock.quantum-limit":
		props.ClockQuantumLimit = value
	case "node.loop.name":
		props.NodeLoopName = value
	case "library.name":
		props.LibraryName = value
	case "client.id":
		props.ClientID = value
	case "object.id":
		props.ObjectID = value
	case "object.serial":
		props.ObjectSerial = value
	case "pulse.attr.maxlength":
		props.PulseAttrMaxlength = value
	case "pulse.attr.tlength":
		props.PulseAttrTlength = value
	case "pulse.attr.prebuf":
		props.PulseAttrPrebuf = value
	case "pulse.attr.minreq":
		props.PulseAttrMinreq = value
	case "node.driver-id":
		props.NodeDriverID = value
	case "module-stream-restore.id":
		props.ModuleStreamRestoreID = value
	default:
		unknownProps[key] = value
	}
}
