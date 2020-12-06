package shalm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/k14s/starlark-go/starlark"
)

// NewHelmChartFunction -
func NewHelmChartFunction(repo Repo, dir string, options ...ChartOption) func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
		var url string
		co := chartOptions(options)
		if err := starlark.UnpackArgs("chart", args, kwargs, "url", &url, "namespace?", &co.namespace, "suffix?", &co.suffix); err != nil {
			return starlark.None, err
		}
		if !(filepath.IsAbs(url) || strings.HasPrefix(url, "http")) {
			url = path.Join(dir, url)
		}
		c, err := repo.Get(thread, url, co.Merge())
		if err != nil {
			return starlark.None, err
		}

		chart := c.(*chartImpl)
		chart.methods["apply"] = chart.wrapNamespace(helmApplyFunction(chart))
		chart.methods["template"] = helmTemplateFunction(chart)
		chart.methods["delete"] = chart.wrapNamespace(helmDeleteFunction(chart))
		return chart, nil
	}
}

func helmApplyFunction(c *chartImpl) starlark.Callable {
	return c.builtin("apply", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
		var k K8sValue
		if err := starlark.UnpackArgs("apply", args, kwargs, "k8s", &k); err != nil {
			return nil, err
		}
		filename, err := values(c)
		if err != nil {
			return starlark.None, err
		}
		return starlark.None, helm(c, k.(*k8sValueImpl).K8s.(*k8sImpl), &K8sOptions{}, "upgrade", "-i", c.GetName(), c.dir, "-f", filename)
	})
}

func helmTemplateFunction(c *chartImpl) starlark.Callable {
	return c.builtin("template", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
		var k K8sValue
		if err := starlark.UnpackArgs("template", args, kwargs, "k8s", &k); err != nil {
			return starlark.None, err
		}
		filename, err := values(c)
		if err != nil {
			return starlark.None, err
		}
		return starlark.String(""), helm(c, k.(*k8sValueImpl).K8s.(*k8sImpl), &K8sOptions{}, "template", c.dir, "-f", filename)
	})
}

func helmDeleteFunction(c *chartImpl) starlark.Callable {
	return c.builtin("apply", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
		var k K8sValue
		if err := starlark.UnpackArgs("apply", args, kwargs, "k8s", &k); err != nil {
			return nil, err
		}
		_ = helm(c, k.(*k8sValueImpl).K8s.(*k8sImpl), &K8sOptions{}, "uninstall", c.GetName())
		return starlark.None, nil
	})
}

func helm(chart *chartImpl, k *k8sImpl, options *K8sOptions, flags ...string) error {
	namespace := k.ns(options)
	if namespace != nil {
		flags = append(flags, "-n", *namespace)
	}
	if options.Timeout > 0 {
		flags = append(flags, "--timeout", fmt.Sprintf("%.0fs", options.Timeout.Seconds()))
	}
	if k.verbose != 0 {
		flags = append(flags, fmt.Sprintf("-v=%d", k.verbose))
	}
	c := k.command
	if c == nil {
		c = exec.CommandContext
	}
	cmd := c(k.ctx, "helm", flags...)
	if !options.Quiet {
		fmt.Println(cmd.String())
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func values(c *chartImpl) (string, error) {
	filename := "/tmp/values"
	data, err := json.Marshal(toGo(c.GetValue()))
	if err != nil {
		return filename, err
	}
	err = ioutil.WriteFile(filename, data, os.FileMode(0755))
	return filename, err

}
