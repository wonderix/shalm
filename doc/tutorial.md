# Tutorial

This tutorial will guide you through the different topics of shalm

## Interoperability with helm

Shalm is designed from ground up to be mostly compatible with helm packages



### Helm to shalm

You deploy almost every helm chart using *shalm*. But *shalm* doesn't support hooks. 

```
helm repo add stable https://charts.helm.sh/stable
helm show chart stable/mariadb
URL=$(curl -s https://charts.helm.sh/stable/index.yaml | yaml2json | jq -r '.entries.mysql | map(select(.deprecated != true  )) | .[0].urls[0]')
shalm apply $URL
kubectl get pods
shalm list -A
shalm delete $URL

```

### Shalm to helm

Shalm chart can be wrapped into helm charts using `shalm package --helm`. Shalm uses `pre-upgrade` hooks to implement this.


```python
# Chart.star
def init(self):
  self.__class__.version = "0.0.1"
```

```bash
rm -rf /tmp/example
mkdir -p /tmp/example
cd /tmp/example
cat > Chart.star <<EOF
def init(self):
  self.__class__.version = "0.0.1"
EOF
shalm package --helm .
helm upgrade -i example example-0.0.1.tgz
helm uninstall example
```


## Templating

In shalm, you can chose between go templating and [ytt](https://get-ytt.io/)

### Go Templating (helm)

Go templating is mostly compatible with helm. All required functions are provided. All attributes of `self` are available as `.Values`

```python
# Chart.star
def init(self):
  self.timeout = 30
```

```yaml
kind: Secret
stringData:
  timeout: {{ .Values.timeout | quote }}
```


```bash
rm -rf /tmp/example
mkdir -p /tmp/example/templates
cd /tmp/example
cat > Chart.star <<EOF
def init(self):
  self.timeout = 30
EOF
cat > templates/secret.yaml <<EOF
kind: Secret
stringData:
  timeout: {{ .Values.timeout | quote }}
EOF
shalm template .
```

#### Methods

You can also call methods from the go template

```python
# Chart.star
def name(self):
  return "my-name"
```
```yaml
kind: Secret
stringData:
  upper: {{ call .Methods.name }}
```

```bash
rm -rf /tmp/example
mkdir -p /tmp/example/templates
cd /tmp/example
cat > Chart.star <<EOF
def name(self):
  return "my-name"
EOF
cat > templates/secret.yaml <<EOF
kind: Secret
stringData:
  upper: {{ call .Methods.name }}
EOF
shalm template .
```


### ytt templating


Go templating is mostly compatible with helm. All required functions are provided. All attributes of `self` are available as `.Values`

```python
# Chart.star
def init(self):
  self.timeout = 30
```

```yaml
kind: Secret
stringData:
  timeout: #@ self.timeout
```


```bash
rm -rf /tmp/example
mkdir -p /tmp/example/ytt-templates
cd /tmp/example
cat > Chart.star <<EOF
def init(self):
  self.timeout = 30
EOF
cat > ytt-templates/secret.yaml <<EOF
kind: Secret
stringData:
  timeout: #@ self.timeout
EOF
shalm template .
```

## Deployment

You can choose between different deployment methods:
* kubectl 
* kapp

### kubectl

```bash
rm -rf /tmp/example
mkdir -p /tmp/example/ytt-templates
cd /tmp/example
cat > Chart.star <<EOF
def init(self):
  pass
EOF
cat > ytt-templates/secret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: test
stringData:
  test: test
EOF
shalm apply .
shalm delete .
```


### Overriding apply/delete


```yaml
# Chart.star
def apply(self,k8s):
   print(k8s.get("services","kubernetes"))
   self.__apply(k8s)
```

```bash
rm -rf /tmp/example
mkdir -p /tmp/example/ytt-templates
cd /tmp/example
cat > Chart.star <<EOF
def apply(self,k8s):
   print(k8s.get("services","kubernetes"))
   self.__apply(k8s)
EOF
shalm apply .
shalm delete .
```

### kapp

```python
cat > Chart.star <<EOF
def apply(self, k8s):
  k8s.tool = "kapp"
  self.__apply(k8s)
```

```bash
rm -rf /tmp/example
mkdir -p /tmp/example/ytt-templates
cd /tmp/example
cat > Chart.star <<EOF
def apply(self, k8s):
  k8s.tool = "kapp"
  self.__apply(k8s)
EOF
cat > ytt-templates/secret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: test
stringData:
  test: test
EOF
shalm apply .
kapp list -A
shalm delete .
```


## Properties

You can define properties, which can be set using the command lines. All items from `values.yaml` inside the (helm) chart are automatically converted to properties

```python
def init(self):
  self.timeout = property(default=30)
```

```yaml
kind: Secret
stringData:
  timeout: #@ self.timeout
```

```bash
rm -rf /tmp/example
mkdir -p /tmp/example/ytt-templates
cd /tmp/example
cat > Chart.star <<EOF
def init(self):
  self.timeout = property(default=30)
EOF
cat > ytt-templates/secret.yaml <<EOF
kind: Secret
stringData:
  timeout: #@ self.timeout
EOF
shalm template . --set timeout=60
```

## Subcharts

## Helm subcharts

## Dependencies

## Configuring Subcharts and Dependencies

## User credentials

## Certificates

## kubernetes client

### Get

### Watch

### Apply

### Delete

## Controller

## Extend

