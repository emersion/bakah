package main

import (
	"encoding/json"
	"io"
	"strings"
)

type File struct {
	Target   map[string]*Target   `json:"target"`
	Group    map[string]*Group    `json:"group"`
	Variable map[string]*Variable `json:"variable"`
	// TODO: function
}

type Target struct {
	Args             map[string]*string  `json:"args"`
	Annotations      []string            `json:"annotations"`
	Attest           []map[string]string `json:"attest"`
	CacheFrom        []Props             `json:"cache-from"`
	CacheTo          []Props             `json:"cache-to"`
	Call             string              `json:"call"`
	Context          string              `json:"context"`
	Contexts         map[string]string   `json:"contexts"`
	Description      string              `json:"description"`
	DockerfileInline string              `json:"dockerfile-inline"`
	Dockerfile       string              `json:"dockerfile"`
	Entitlements     []string            `json:"entitlements"`
	Inherits         []string            `json:"inherits"`
	Labels           map[string]*string  `json:"labels"`
	Matrix           map[string][]string `json:"matrix"`
	Name             string              `json:"string"`
	Network          string              `json:"network"`
	NoCacheFilter    []string            `json:"no-cache-filter"`
	NoCache          bool                `json:"no-cache"`
	Output           []Props             `json:"output"`
	Platforms        []string            `json:"platforms"`
	Pull             string              `json:"pull"`
	Secret           []Props             `json:"secret"`
	ShmSize          string              `json:"shm-size"`
	SSH              []Props             `json:"ssh"`
	Tags             []string            `json:"tags"`
	Target           string              `json:"target"`
	Ulimits          []string            `json:"ulimits"`
}

type Group struct {
	Targets []string `json:"targets"`
}

type Variable struct {
	// TODO: unset vs null vs empty string
	Default *string `json:"default"`
}

type Props map[string]string

func (out *Props) UnmarshalJSON(b []byte) error {
	props := make(map[string]string)

	if len(b) > 0 && b[0] == '"' {
		var raw string
		if err := json.Unmarshal(b, &raw); err != nil {
			return err
		}
		for _, kv := range strings.Split(raw, ",") {
			k, v, _ := strings.Cut(kv, "=")
			props[k] = v
		}
	} else {
		if err := json.Unmarshal(b, &props); err != nil {
			return err
		}
	}

	*out = Props(props)
	return nil
}

func Decode(r io.Reader) (*File, error) {
	var f *File
	if err := json.NewDecoder(r).Decode(&f); err != nil {
		return nil, err
	}
	return f, nil
}
