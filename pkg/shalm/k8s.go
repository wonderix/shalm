package shalm

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/labels"
)

//go:generate ./generate_fake.sh

// K8sOptions common options for calls to k8s
type K8sOptions struct {
	ClusterScoped  bool
	Namespace      string
	Timeout        time.Duration
	IgnoreNotFound bool
	Quiet          bool
	Tool           Tool
}

// ListOptions -
type ListOptions struct {
	LabelSelector labels.Selector
	AllNamespaces bool
}

// K8sReader kubernetes reader API
type K8sReader interface {
	Host() string
	Get(kind string, name string, options *K8sOptions) (*Object, error)
	List(kind string, options *K8sOptions, listOptions *ListOptions) (*Object, error)
	IsNotExist(err error) bool
}

// K8s kubernetes API
type K8s interface {
	K8sReader
	ForSubChart(namespace string, app string, version *semver.Version, children int) K8s
	Inspect() string
	Watch(kind string, name string, options *K8sOptions) ObjectStream
	RolloutStatus(kind string, name string, options *K8sOptions) error
	Wait(kind string, name string, condition string, options *K8sOptions) error
	DeleteObject(kind string, name string, options *K8sOptions) error
	Apply(output ObjectStream, options *K8sOptions) error
	Delete(output ObjectStream, options *K8sOptions) error
	Patch(kind string, name string, pt types.PatchType, patch string, options *K8sOptions) (*Object, error)
	CreateOrUpdate(obj *Object, mutate func(obj *Object) error, options *K8sOptions) (*Object, error)
	DeleteByName(kind string, name string, options *K8sOptions) error
	ConfigContent() *string
	ForConfig(config string) (K8s, error)
	WithContext(ctx context.Context) K8s
	Progress(progress int)
	Tool() Tool
	SetTool(tool Tool)
}

// ProgressSubscription -
type ProgressSubscription = func(progress int)

// Tool -
type Tool int

const (
	// ToolKubectl -
	ToolKubectl = iota
	// ToolKapp -
	ToolKapp
)

func (t Tool) String() string {
	return [...]string{"kubectl", "kapp"}[t]
}

// Set -
func (t *Tool) Set(val string) error {
	switch val {
	case "kubectl":
		*t = ToolKubectl
	case "kapp":
		*t = ToolKapp
	case "":
		*t = ToolKubectl
	default:
		return fmt.Errorf("invalid Tool %s", val)
	}
	return nil
}

// Type -
func (t *Tool) Type() string {
	return "tool"
}

type regexpVar struct {
	re **regexp.Regexp
}

func (r *regexpVar) String() string {
	if r.re != nil && *r.re != nil {
		return (*r.re).String()
	}
	return "nil"
}

// Set -
func (r *regexpVar) Set(val string) (err error) {
	*r.re, err = regexp.Compile(val)
	return
}

// Type -
func (r *regexpVar) Type() string {
	return "regexp"
}

// K8sConfigs -
type K8sConfigs struct {
	tool                 Tool
	progressSubscription ProgressSubscription
	kubeConfig           string
	progress             int
	verbose              int
}

// K8sConfig -
type K8sConfig func(options *K8sConfigs) error

// WithTool -
func WithTool(value Tool) K8sConfig {
	return func(options *K8sConfigs) error { options.tool = value; return nil }
}

// WithProgressSubscription -
func WithProgressSubscription(value ProgressSubscription) K8sConfig {
	return func(options *K8sConfigs) error { options.progressSubscription = value; return nil }
}

// WithVerbose -
func WithVerbose(value int) K8sConfig {
	return func(options *K8sConfigs) error { options.verbose = value; return nil }
}

// WithKubeConfigContent -
func WithKubeConfigContent(value string) K8sConfig {
	if value == "" {
		return func(options *K8sConfigs) (err error) { return nil }
	}
	return func(options *K8sConfigs) (err error) { options.kubeConfig, err = kubeConfigFromContent(value); return }
}

// Progress -
func (v *K8sConfigs) Progress(progress int) {
	if v.progressSubscription != nil {

		progress = progress / 5 * 5
		if progress == v.progress {
			return
		}
		v.progress = progress
		v.progressSubscription(progress)
	}
}

// Merge -
func (v *K8sConfigs) Merge() K8sConfig {
	return func(o *K8sConfigs) error {
		*o = *v
		return nil
	}
}

// Tool -
func (v *K8sConfigs) Tool() Tool {
	return v.tool
}

// SetTool -
func (v *K8sConfigs) SetTool(tool Tool) {
	v.tool = tool
}

// AddFlags -
func (v *K8sConfigs) AddFlags(flagsSet *pflag.FlagSet) {
	flagsSet.VarP(&v.tool, "tool", "t", "Tool to do the installation. Possible values kubectl (default) and kapp")
	flagsSet.IntVarP(&v.verbose, "verbose", "v", 0, "Set kubectl verbose level")
}

// NewK8s create new instance to interact with kubernetes
func NewK8s(configs ...K8sConfig) (K8s, error) {
	var err error
	result := &k8sImpl{ctx: context.Background(), app: "root"}
	for _, config := range configs {
		if err = config(&result.K8sConfigs); err != nil {
			return nil, err
		}
	}
	return result.connect()
}

func (k *k8sImpl) connect() (K8s, error) {
	config, err := configKube(k.kubeConfig)
	if err != nil {
		return nil, err
	}
	k.client, err = newK8sClient(config)
	if err != nil {
		return nil, err
	}
	if len(config.Host) != 0 {
		u, err := url.Parse(config.Host)
		if err != nil {
			k.host = config.Host
		} else {
			k.host = u.Hostname()
		}
	}
	return k, nil
}

// k8sImpl -
type k8sImpl struct {
	K8sConfigs
	namespace        string
	command          func(ctx context.Context, name string, arg ...string) *exec.Cmd
	localProgress    int
	childrenProgress []int
	children         int
	app              string
	version          *semver.Version
	client           *k8sClient
	host             string
	ctx              context.Context
}

var (
	_ K8s = (*k8sImpl)(nil)
)

func (k *k8sImpl) Inspect() string {
	if len(k.kubeConfig) != 0 {
		return "kubeConfig = " + k.kubeConfig + " namespace = " + k.namespace
	}
	return "namespace = " + k.namespace
}

func (k *k8sImpl) Host() string {
	return k.host
}

func (k *k8sImpl) progressCb(matched int, count int) {
	if count == 0 || matched > count {
		return
	}
	k.localProgress = matched * 90 / count
	k.reportProgress()
}

// Apply -
func (k *k8sImpl) Apply(output ObjectStream, options *K8sOptions) (err error) {
	if k.tool == ToolKapp {
		writer, stream := prepareKapp(output, false, k.objMapper(), k.progressCb)
		err = runWithStdin(k.kapp("deploy", options, "-f", "-"), stream, writer, k.verbose)
	} else {
		writer, stream := prepareKubectl(output, false, k.objMapper(), k.progressCb)
		err = runWithStdin(k.kubectl("apply", options, "-f", "-"), stream, writer, k.verbose)
	}
	return err
}

func (k *k8sImpl) reportProgress() {
	sum := k.localProgress
	for _, p := range k.childrenProgress {
		sum += p
	}
	k.Progress(sum / (k.children + 1))
}

func (k *k8sImpl) addProgressSubscription() ProgressSubscription {
	if k.progressSubscription == nil {
		return nil
	}
	index := len(k.childrenProgress)
	k.childrenProgress = append(k.childrenProgress, 0)
	return func(progress int) {
		k.childrenProgress[index] = progress
		k.reportProgress()
	}
}

func (k *k8sImpl) clone() *k8sImpl {
	return &k8sImpl{namespace: k.namespace, app: k.app, version: k.version, client: k.client, host: k.host, ctx: k.ctx,
		K8sConfigs: K8sConfigs{
			progressSubscription: k.addProgressSubscription(),
			kubeConfig:           k.kubeConfig,
			tool:                 ToolKubectl,
			verbose:              k.verbose,
		}}
}

func (k *k8sImpl) ForSubChart(namespace string, app string, version *semver.Version, children int) K8s {
	result := k.clone()
	result.namespace = namespace
	result.app = app
	result.version = version
	result.children = children
	return result
}

func (k *k8sImpl) WithContext(ctx context.Context) K8s {
	k.ctx = ctx
	return k
}

// Delete -
func (k *k8sImpl) Delete(output ObjectStream, options *K8sOptions) (err error) {
	if k.tool == ToolKapp {
		writer, _ := prepareKapp(output, false, k.objMapper(), k.progressCb)
		err = runWithStdin(k.kapp("delete", options), func(w io.Writer) error { return nil }, writer, k.verbose)
	} else {
		writer, stream := prepareKubectl(output, true, k.objMapper(), k.progressCb)
		err = runWithStdin(k.kubectl("delete", options, "--ignore-not-found", "-f", "-"), stream, writer, k.verbose)
	}
	if err != nil && k.IsNotExist(err) {
		err = nil
	}
	return err
}

var invalidValueRegex = regexp.MustCompile("[^a-zA-Z0-9\\-_\\.]")

func (k *k8sImpl) objMapper() func(obj *Object) *Object {
	return func(obj *Object) *Object {
		obj.setDefaultNamespace(k.namespace)
		if obj.MetaData.Labels == nil {
			obj.MetaData.Labels = make(map[string]string)
		}
		obj.MetaData.Labels["shalm.wonderix.github.com/app"] = invalidValueRegex.ReplaceAllString(k.app, "")
		obj.MetaData.Labels["shalm.wonderix.github.com/version"] = invalidValueRegex.ReplaceAllString(k.version.String(), "")
		return obj
	}
}

// Delete -
func (k *k8sImpl) DeleteObject(kind string, name string, options *K8sOptions) error {
	return run(k.kubectl("delete", options, kind, name, "--ignore-not-found"))
}

// RolloutStatus -
func (k *k8sImpl) RolloutStatus(kind string, name string, options *K8sOptions) error {
	start := time.Now()
	for {
		err := run(k.kubectl("rollout", options, "status", kind, name))
		if err == nil {
			return nil
		}
		if !k.IsNotExist(err) {
			return err
		}
		if options.Timeout > 0 {
			if time.Since(start) > options.Timeout {
				return fmt.Errorf("Timeout during waiting for %s %s", kind, name)
			}
		}
		time.Sleep(10 * time.Second)
	}
}

func (k *k8sImpl) Wait(kind string, name string, condition string, options *K8sOptions) error {
	return run(k.kubectl("wait", options, kind, name, "--for", condition))
}

func wrapError(err error) error {
	statusError, ok := err.(*k8serrors.StatusError)
	if ok {
		return fmt.Errorf("HTTP Status: %d, Message: %s", statusError.ErrStatus.Code, statusError.ErrStatus.Message)
	}
	return err
}

func ignoreNotFound(obj *Object, err error, options *K8sOptions) (*Object, error) {
	if err == nil {
		return obj, nil
	}
	if options.IgnoreNotFound && k8serrors.IsNotFound(err) {
		return nil, nil
	}
	return nil, wrapError(err)
}

// Get -
func (k *k8sImpl) Get(kind string, name string, options *K8sOptions) (*Object, error) {
	if k.client != nil {
		obj, err := k.client.Get().Namespace(k.ns(options)).Resource(kind).Name(name).Do().Get()
		if err == nil {
			return obj, nil
		}
		_, ok := err.(*errUnknownResource)
		if !ok {
			return ignoreNotFound(obj, err, options)
		}
	}

	cmd := k.kubectl("get", options, kind, name, "-o", "json")
	buffer := &bytes.Buffer{}
	cmd.Stdout = buffer
	if err := run(cmd); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(buffer)
	var result Object
	err := decoder.Decode(&result)
	return &result, err
}

// Patch -
func (k *k8sImpl) Patch(kind string, name string, pt types.PatchType, patch string, options *K8sOptions) (*Object, error) {
	if k.client == nil {
		return nil, errors.New("Not connected")
	}
	obj, err := k.client.Patch(pt).Namespace(k.ns(options)).Resource(kind).Name(name).Body([]byte(patch)).Do().Get()
	return ignoreNotFound(obj, err, options)
}

func (k *k8sImpl) CreateOrUpdate(obj *Object, mutate func(obj *Object) error, options *K8sOptions) (*Object, error) {
	if k.client == nil {
		return nil, errors.New("Not connected")
	}
	var req request
	old, err := k.Get(obj.Kind, obj.MetaData.Name, options)
	if err != nil {
		if !k.IsNotExist(err) {
			return nil, err
		}
		req = k.client.Post().Namespace(k.ns(options)).Resource(obj.Kind)
	} else {
		obj = old
		req = k.client.Put().Namespace(k.ns(options)).Resource(obj.Kind).Name(obj.MetaData.Name)
	}
	err = mutate(obj)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return req.Body(body).Do().Get()

}

func (k *k8sImpl) DeleteByName(kind string, name string, options *K8sOptions) error {
	if k.client == nil {
		return errors.New("Not connected")
	}
	err := k.client.Delete().Namespace(k.ns(options)).Resource(kind).Name(name).Do().Error()
	if err != nil {
		if options.IgnoreNotFound && k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

// List -
func (k *k8sImpl) List(kind string, options *K8sOptions, listOptions *ListOptions) (*Object, error) {
	flags := []string{kind, "-o", "json"}
	if listOptions.AllNamespaces {
		options.ClusterScoped = true
		flags = append(flags, "-A")
	}
	requirements, selectable := listOptions.LabelSelector.Requirements()
	if selectable {
		for _, req := range requirements {
			flags = append(flags, "-l", req.Key()+string(req.Operator())+req.Values().List()[0])
		}
	}
	cmd := k.kubectl("get", options, flags...)
	buffer := &bytes.Buffer{}
	cmd.Stdout = buffer
	if err := run(cmd); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(buffer)
	var result Object
	err := decoder.Decode(&result)
	return &result, err
}

// Watch -
func (k *k8sImpl) Watch(kind string, name string, options *K8sOptions) ObjectStream {
	return func(writer ObjectWriter) error {
		cmd := k.kubectl("get", options, kind, name, "-o", "json", "--watch")
		reader, w := io.Pipe()
		cmd.Stdout = w
		defer reader.Close()
		err := cmd.Start()
		if err != nil {
			return err
		}
		decoder := json.NewDecoder(reader)
		for {
			var obj Object
			if decoder.Decode(&obj) != nil {
				break
			}
			err := writer(&obj)
			if err != nil {
				if _, ok := err.(*CancelObjectStream); ok {
					return nil
				}
				return err
			}
		}
		return cmd.Wait()
	}
}

// IsNotExist -
func (k *k8sImpl) IsNotExist(err error) bool {
	return strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "no matches for kind") ||
		strings.Contains(err.Error(), "the server could not find the requested resource")
}

// ConfigContent -
func (k *k8sImpl) ConfigContent() *string {
	if len(k.kubeConfig) == 0 {
		return nil
	}
	data, err := ioutil.ReadFile(k.kubeConfig)
	if err != nil {
		return nil
	}
	content := string(data)
	return &content
}

// ForConfig -
func (k *k8sImpl) ForConfig(config string) (K8s, error) {
	result := k.clone()
	err := WithKubeConfigContent(config)(&result.K8sConfigs)
	if err != nil {
		return nil, err
	}
	return result.connect()
}

func run(cmd *exec.Cmd) error {
	buffer := bytes.Buffer{}
	cmd.Stderr = io.MultiWriter(&buffer, os.Stderr)
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, string(buffer.Bytes()))
	}
	return nil

}

func (k *k8sImpl) ns(options *K8sOptions) *string {
	if options.ClusterScoped {
		return nil
	}
	namespace := k.namespace
	if options.Namespace != "" {
		namespace = options.Namespace
	}
	return &namespace
}

func (k *k8sImpl) kubectl(command string, options *K8sOptions, flags ...string) *exec.Cmd {
	if len(k.kubeConfig) != 0 {
		flags = append([]string{command, "--kubeconfig", k.kubeConfig}, flags...)
	} else {
		flags = append([]string{command}, flags...)
	}
	namespace := k.ns(options)
	if namespace != nil {
		flags = append(flags, "-n", *namespace)
	}
	if options.IgnoreNotFound {
		flags = append(flags, "--ignore-not-found")
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
	cmd := c(k.ctx, "kubectl", flags...)
	if options.Quiet {
		cmd.Stdout = &bytes.Buffer{}
	} else {
		fmt.Println(cmd.String())
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	return cmd
}

func (k *k8sImpl) kapp(command string, options *K8sOptions, flags ...string) *exec.Cmd {
	if len(k.kubeConfig) != 0 {
		flags = append([]string{command, "--kubeconfig", k.kubeConfig}, flags...)
	} else {
		flags = append([]string{command}, flags...)
	}
	namespace := k.ns(options)
	if namespace != nil {
		flags = append(flags, "-n", *namespace)
	} else if len(k.kubeConfig) != 0 {
		flags = append(flags, "-n", "default")
	}
	if options.Timeout > 0 {
		flags = append(flags, "--wait-timeout", fmt.Sprintf("%.0fs", options.Timeout.Seconds()))
	}
	c := k.command
	if c == nil {
		c = exec.CommandContext
	}
	flags = append(flags, "-a", k.app, "-y")
	cmd := c(k.ctx, "kapp", flags...)
	fmt.Println(cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func runWithStdin(cmd *exec.Cmd, output func(io.Writer) error, progress io.Writer, verbose int) error {

	writer, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	buffer := bytes.Buffer{}
	cmd.Stderr = io.MultiWriter(&buffer, cmd.Stderr)
	if progress != nil {
		cmd.Stdout = io.MultiWriter(cmd.Stdout, progress)
	}
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("error starting %s: %s", cmd.String(), err.Error())
	}
	w := &writeCounter{writer: writer}
	if verbose >= 8 {
		err = output(io.MultiWriter(w, os.Stderr))
	} else {
		err = output(w)
	}
	if err != nil {
		return err
	}
	writer.Close()
	err = cmd.Wait()
	if err != nil {
		if w.counter == 0 {
			return nil
		}
		return errors.Wrapf(err, "error running %s: %s", cmd.String(), buffer.String())
	}
	return nil

}

func prepare(in ObjectStream, reverse bool, mapper func(obj *Object) *Object) Stream {
	return in.
		Map(mapper).
		Sort(compare, reverse).
		Encode()
}

var kubectlRegexp = regexp.MustCompile(`(configured|unchanged|created|deleted)$`)

func prepareKubectl(in ObjectStream, reverse bool, mapper func(obj *Object) *Object, progress func(matched int, count int)) (io.Writer, Stream) {
	count := 0
	matched := 0
	writer := &lineWriter{line: func(line string) {
		if kubectlRegexp.MatchString(line) {
			matched++
			progress(matched, count)
		}

	}}
	mapper2 := func(obj *Object) *Object {
		count++
		return mapper(obj)
	}
	return writer, prepare(in, reverse, mapper2)
}

var kappRegexp = regexp.MustCompile(`waiting.*\[(\d+)/(\d+)\s+done\]`)

func prepareKapp(in ObjectStream, reverse bool, mapper func(obj *Object) *Object, progress func(matched int, count int)) (io.Writer, Stream) {
	writer := &lineWriter{line: func(line string) {
		match := kappRegexp.FindStringSubmatch(line)
		if len(match) == 3 {
			count, err := strconv.Atoi(match[2])
			if err != nil || count == 0 {
				return
			}
			matched, err := strconv.Atoi(match[1])
			if err != nil {
				return
			}
			progress(matched, count)
		}
	}}
	return writer, prepare(in, reverse, mapper)
}

func compare(o1 *Object, o2 *Object) int {
	diff := o1.kindOrdinal() - o2.kindOrdinal()
	if diff != 0 {
		return diff
	}
	return strings.Compare(o1.MetaData.Name, o2.MetaData.Name)
}

type lineWriter struct {
	buffer bytes.Buffer
	line   func(line string)
}

func (l *lineWriter) Write(data []byte) (int, error) {
	for _, d := range data {

		if d == '\n' {
			l.line(l.buffer.String())
			l.buffer.Reset()
		} else {
			l.buffer.WriteByte(d)
		}
	}
	return len(data), nil
}

func digestString(s string) string {
	slice := md5.Sum([]byte(s))
	return hex.EncodeToString(slice[:])
}
