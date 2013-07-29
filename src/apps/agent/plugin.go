package main

import (
	log "code.google.com/p/log4go"
	"fmt"
	"github.com/errplane/errplane-go"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	. "utils"
)

type PluginStateOutput int

func (p *PluginStateOutput) String() string {
	switch *p {
	case OK:
		return "Ok"
	case WARNING:
		return "Warning"
	case CRITICAL:
		return "Critical"
	case UNKNOWN:
		return "unknown"
	default:
		panic(fmt.Errorf("WTF unknown state %d", *p))
	}
}

const (
	OK PluginStateOutput = iota
	WARNING
	CRITICAL
	UNKNOWN
)

type PluginOutput struct {
	state   PluginStateOutput
	msg     string
	metrics map[string]float64
}

// handles running plugins

func monitorPlugins(ep *errplane.Errplane) {
	for {
		log.Debug("Iterating through %d plugins", len(Plugins))

		for _, plugin := range Plugins {
			log.Debug("Running command %s", plugin.Cmd)
			go runPlugin(ep, plugin)
		}

		time.Sleep(Sleep)
	}
}

func runPlugin(ep *errplane.Errplane, plugin *Plugin) {
	cmdParts := strings.Fields(plugin.Cmd)
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("Cannot run plugin %s. Error: %s", plugin.Cmd, err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Error("Cannot run plugin %s. Error: %s", plugin.Cmd, err)
		return
	}

	ch := make(chan error)
	go killPlugin(plugin, cmd, ch)

	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		log.Error("Error while reading output from plugin %s. Error: %s", plugin.Cmd, err)
		ch <- err
		return
	}

	lines := strings.Split(string(output), "\n")

	err = cmd.Wait()
	ch <- err

	if len(lines) > 0 {
		log.Debug("output of plugin %s is %s", plugin.Cmd, lines[0])
		firstLine := lines[0]
		output, err := parsePluginOutput(plugin, cmd.ProcessState, firstLine)
		if err != nil {
			log.Error("Cannot parse plugin %s output. Output: %s. Error: %s", plugin.Cmd, firstLine, err)
		}

		// status are printed to plugins.<plugin-nam>.status with a value of 1 and dimension status that is either ok, warning, critical or unknown
		// other metrics are written to plugins.<plugin-name>.<metric-name> with the given value
		// all metrics have the host name as a dimension

		report(ep, fmt.Sprintf("plugins.%s.status", plugin.Name), 1.0, time.Now(), errplane.Dimensions{
			"host":       Hostname,
			"status":     output.state.String(),
			"status_msg": output.msg,
		}, nil)

		for name, value := range output.metrics {
			report(ep, fmt.Sprintf("plugins.%s.%s", plugin.Name, name), value, time.Now(), errplane.Dimensions{"host": Hostname}, nil)
		}
	}
}

func parsePluginOutput(plugin *Plugin, cmdState *os.ProcessState, firstLine string) (*PluginOutput, error) {
	firstLine = strings.TrimSpace(firstLine)

	statusAndMetrics := strings.Split(firstLine, "|")
	if len(statusAndMetrics) != 2 {
		return nil, fmt.Errorf("First line format doesn't match what the agent expects. See the docs for more details")
	}

	status, metrics := statusAndMetrics[0], statusAndMetrics[1]

	// FIXME: linux specific
	exitStatus := cmdState.Sys().(syscall.WaitStatus).ExitStatus()

	metricsMap := make(map[string]float64)

	for _, metric := range strings.Fields(metrics) {
		nameAndValue := strings.Split(metric, "=")
		if len(nameAndValue) != 2 {
			return nil, fmt.Errorf("First line format doesn't match what the agent expects. See the docs for more details")
		}

		name, value := nameAndValue[0], nameAndValue[1]
		var err error
		metricsMap[name], err = strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("Invalid numeric value in plugin output. Error: %s", err)
		}
	}

	return &PluginOutput{PluginStateOutput(exitStatus), status, metricsMap}, nil
}

func killPlugin(plugin *Plugin, cmd *exec.Cmd, ch chan error) {
	select {
	case err := <-ch:
		if exitErr, ok := err.(*exec.ExitError); ok && !exitErr.Exited() {
			log.Error("plugin %s didn't die gracefully. Killing it.", plugin.Cmd)
			cmd.Process.Kill()
		}
	case <-time.After(Sleep):
		err := cmd.Process.Kill()
		if err != nil {
			log.Error("Cannot kill plugin %s. Error: %s", plugin.Cmd, err)
		}
		log.Error("Plugin %s killed because it took more than %s to execute", plugin.Cmd, Sleep)
	}
}
