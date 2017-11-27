package ops

import (
	"fmt"
	"strings"

	"github.com/cppforlife/go-patch/patch"

	yaml "gopkg.in/yaml.v2"
)

type Op struct {
	Type  string      `yaml:"type"`
	Path  string      `yaml:"path"`
	Value interface{} `yaml:"value"`
}

var marshal = yaml.Marshal

func ApplyOp(manifest string, op Op) (string, error) {
	return ApplyOps(manifest, []Op{op})
}

func ApplyOps(manifest string, ops []Op) (string, error) {
	var doc interface{}
	err := yaml.Unmarshal([]byte(manifest), &doc)
	if err != nil {
		return "", err
	}

	goPatchOps := patch.Ops{}
	for _, op := range ops {
		goPatchOp, err := makeGoPatchOp(op)
		if err != nil {
			return "", err
		}

		goPatchOps = append(goPatchOps, goPatchOp)
	}

	doc, err = goPatchOps.Apply(doc)
	if err != nil {
		// not tested
		return "", err
	}

	manifestYAML, err := marshal(doc)
	if err != nil {
		return "", err
	}

	return strings.Trim(string(manifestYAML), "\n"), nil
}

func FindOp(manifest, path string) (interface{}, error) {
	var doc interface{}
	err := yaml.Unmarshal([]byte(manifest), &doc)
	if err != nil {
		return "", err
	}

	pointerPath, err := patch.NewPointerFromString(path)
	if err != nil {
		return "", err
	}

	goPatchOps := patch.Ops{
		patch.FindOp{
			Path: pointerPath,
		},
	}

	doc, err = goPatchOps.Apply(doc)
	if err != nil {
		// not tested
		return "", err
	}

	return doc, nil
}

func makeGoPatchOp(op Op) (patch.Op, error) {
	switch op.Type {
	case "replace":
		path, err := patch.NewPointerFromString(op.Path)
		if err != nil {
			return nil, err
		}

		return patch.ReplaceOp{
			Path:  path,
			Value: op.Value,
		}, nil
	case "remove":
		path, err := patch.NewPointerFromString(op.Path)
		if err != nil {
			return nil, err
		}

		return patch.RemoveOp{
			Path: path,
		}, nil
	default:
		return nil, fmt.Errorf("op type %s not supported by destiny", op.Type)
	}
}
