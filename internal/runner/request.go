package runner

// This file contains the implementation of a message parser for requests
// arriving from studioml queues formatted using JSON.
//
// To parse and unparse this JSON data use the following ...
//
//    r, err := UnmarshalRequest(bytes)
//    bytes, err = r.Marshal()

import (
	"bytes"
	"encoding/gob"
	"encoding/json"

	"github.com/dustin/go-humanize"

	"github.com/go-stack/stack"
	"github.com/karlmutch/errors"
)

// Resource describes the needed resources for a runner task in a data structure that can be
// marshalled as json
//
type Resource struct {
	Cpus   uint   `json:"cpus"`
	Gpus   uint   `json:"gpus"`
	Hdd    string `json:"hdd"`
	Ram    string `json:"ram"`
	GpuMem string `json:"gpuMem"`
}

// Fit determines is a supplied resource description acting as a request can
// be satisfied by the receiver resource
//
func (l *Resource) Fit(r *Resource) (didFit bool, err errors.Error) {

	lRam, errGo := humanize.ParseBytes(l.Ram)
	if errGo != nil {
		return false, errors.New("left side RAM could not be parsed").With("stack", stack.Trace().TrimRuntime())
	}

	rRam, errGo := humanize.ParseBytes(r.Ram)
	if errGo != nil {
		return false, errors.New("right side RAM could not be parsed").With("stack", stack.Trace().TrimRuntime())
	}

	lHdd, errGo := humanize.ParseBytes(l.Hdd)
	if errGo != nil {
		return false, errors.New("left side Hdd could not be parsed").With("stack", stack.Trace().TrimRuntime())
	}

	rHdd, errGo := humanize.ParseBytes(r.Hdd)
	if errGo != nil {
		return false, errors.New("right side Hdd could not be parsed").With("stack", stack.Trace().TrimRuntime())
	}

	lGpuMem, errGo := humanize.ParseBytes(l.GpuMem)
	// GpuMem is optional so handle the case when it does not parse and is empty
	if 0 != len(l.GpuMem) {
		if errGo != nil {
			return false, errors.New("left side gpuMem could not be parsed").With("left_mem", l.GpuMem).With("stack", stack.Trace().TrimRuntime())
		}
	}

	rGpuMem, errGo := humanize.ParseBytes(r.GpuMem)
	// GpuMem is optional so handle the case when it does not parse and is empty
	if 0 != len(r.GpuMem) {
		if errGo != nil {
			return false, errors.New("right side gpuMem could not be parsed").With("right", r.GpuMem).With("stack", stack.Trace().TrimRuntime())
		}
	}

	return l.Cpus <= r.Cpus && l.Gpus <= r.Gpus && lHdd <= rHdd && lRam <= rRam && lGpuMem <= rGpuMem, nil
}

// Clone will deep copy a resource and return the copy
//
func (l *Resource) Clone() (r *Resource) {

	var mod bytes.Buffer
	enc := gob.NewEncoder(&mod)
	dec := gob.NewDecoder(&mod)

	if err := enc.Encode(l); err != nil {
		return nil
	}

	r = &Resource{}
	if err := dec.Decode(r); err != nil {
		return nil
	}
	return r
}

// Config is a marshalled data structure used with studioml requests for defining the
// configuration of an environment used to run jobs
type Config struct {
	Cloud                  interface{}       `json:"cloud"`
	Database               Database          `json:"database"`
	SaveWorkspaceFrequency string            `json:"saveWorkspaceFrequency"`
	Lifetime               string            `json:"experimentLifetime"`
	Verbose                string            `json:"verbose"`
	Env                    map[string]string `json:"env"`
	Pip                    []string          `json:"pip"`
	Runner                 RunnerCustom      `json:"runner"`
}

// RunnerCustom defines a custom type of resource used by the go runner to implement a slack
// notification mechanism
//
type RunnerCustom struct {
	SlackDest string `json:"slack_destination"`
}

// Database marshalls the studioML database specification for experiment meta data
type Database struct {
	ApiKey            string `json:"apiKey"`
	AuthDomain        string `json:"authDomain"`
	DatabaseURL       string `json:"databaseURL"`
	MessagingSenderId int64  `json:"messagingSenderId"`
	ProjectId         string `json:"projectId"`
	StorageBucket     string `json:"storageBucket"`
	Type              string `json:"type"`
	UseEmailAuth      bool   `json:"use_email_auth"`
}

// Experiment marshalls the studioML experiment meta data
type Experiment struct {
	Args               []string            `json:"args"`
	Artifacts          map[string]Artifact `json:"artifacts"`
	Filename           string              `json:"filename"`
	Git                interface{}         `json:"git"`
	Info               Info                `json:"info"`
	Key                string              `json:"key"`
	Metric             interface{}         `json:"metric"`
	Project            interface{}         `json:"project"`
	Pythonenv          []string            `json:"pythonenv"`
	PythonVer          int64               `json:"pythonver"`
	Resource           Resource            `json:"resources_needed"`
	Status             string              `json:"status"`
	TimeAdded          float64             `json:"time_added"`
	MaxDuration        string              `json:"max_duration"`
	TimeFinished       interface{}         `json:"time_finished"`
	TimeLastCheckpoint interface{}         `json:"time_last_checkpoint"`
	TimeStarted        interface{}         `json:"time_started"`
}

// Request marshalls the requests made by studioML under which all of the other
// meta data can be found
type Request struct {
	Config     Config     `json:"config"`
	Experiment Experiment `json:"experiment"`
}

// Info is a marshalled item from the studioML experiment definition that
// is ignored by the go runner and so is stubbed out
type Info struct {
}

// Artifact is a marshalled component of a StudioML experiment definition that
// is used to encapsulate files and other external data sources
// that the runner retrieve and/or upload as the experiment progresses
type Artifact struct {
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	Hash      string `json:"hash,omitempty"`
	Local     string `json:"local,omitempty"`
	Mutable   bool   `json:"mutable"`
	Unpack    bool   `json:"unpack"`
	Qualified string `json:"qualified"`
}

// UnmarshalRequest takes an encoded StudioML request and extracts it
// into go data structures used by the go runner
//
func UnmarshalRequest(data []byte) (r *Request, err errors.Error) {
	r = &Request{}
	errGo := json.Unmarshal(data, r)
	if errGo != nil {
		return nil, errors.Wrap(errGo).With("stack", stack.Trace().TrimRuntime())
	}
	return r, nil
}

// Marshal takes the go data structure used to define a StudioML experiment
// request and serializes it as json to the byte array
//
func (r *Request) Marshal() ([]byte, error) {
	return json.Marshal(r)
}
