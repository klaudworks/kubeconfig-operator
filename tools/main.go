//go:build tools

package main

import (
	_ "github.com/hashicorp/go-getter/cmd/go-getter"
	_ "github.com/rinchsan/gosimports/cmd/gosimports"
	_ "sigs.k8s.io/controller-runtime/tools/setup-envtest"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "sigs.k8s.io/kustomize/kustomize/v5"
)
