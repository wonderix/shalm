#!/usr/bin/env bash
set -eu
unset DOLLAR
go run github.com/maxbrunsfeld/counterfeiter/v6 -o fake_k8s_test.go . K8s
sed -e 's|shalm\.||g' -i fake_k8s_test.go
sed -e 's|^.*"github.*shalm".*$||g' -i  fake_k8s_test.go

