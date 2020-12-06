## Reference

The following section describes the available methods inside `Chart.star`

### Chart

#### `chart("<url>",namespace=namespace, ...)`

An new chart is created.  
If no namespace is given, the namespace is inherited from the parent chart.

| Parameter   | Description                                                                                                                                                                                                                                  |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `url`       | The chart is loaded from the given url. The url can be relative.  In this case the chart is loaded from a path relative to the current chart location.                                                                                       |
| `namespace` | If no namespace is given, the namespace is inherited from the parent chart.                                                                                                                                                                  |
| `...`       | Additional parameters are passed to the `init` method of the corresponding chart.                                                                                                                                                            |

#### `chart.apply(k8s)`

Applies the chart recursive to k8s. This method can be overwritten.

| Parameter | Description |
| --------- | ----------- |
| `8s`      | See below   |

#### `chart.__apply(k8s, timeout=0, glob=pattern)`

Applies the chart to k8s without recursion. This should only be used within `apply`

| Parameter | Description                                                              |
| --------- | ------------------------------------------------------------------------ |
| `k8s`     | See below                                                                |
| `timeout` | Timeout passed to `kubectl apply`. A timeout of zero means wait forever. |
| `glob`    | Pattern used to find the templates. Default is "*.yaml"                  |

#### `chart.delete(k8s)`

Deletes the chart recursive from k8s. This method can be overwritten.

| Parameter | Description |
| --------- | ----------- |
| `k8s`     | See below   |

#### `chart.__delete(k8s, timeout=0, glob=pattern)`

Deletes the chart from k8s without recursion. This should only be used within `delete`

| Parameter | Description                                                              |
| --------- | ------------------------------------------------------------------------ |
| `k8s`     | See below                                                                |
| `timeout` | Timeout passed to `kubectl apply`, A timeout of zero means wait forever. |
| `glob`    | Pattern used to find the templates. Default is `"*.y*ml"`                |

#### `chart.template(glob=pattern)`

Renders helm templates and returns a `stream`. The default implementation of this methods renders

* all templates in directory `templates` using `helm`
* all templates in directory `ytt-templates` using `ytt`

It's possible to override this method.


| Parameter | Description                                               |
| --------- | --------------------------------------------------------- |
| `glob`    | Pattern used to find the templates. Default is `"*.y*ml"` |

#### `chart.helm(dir,glob=pattern)`

Renders helm templates and returns a `stream`.

| Parameter | Description                                               |
| --------- | --------------------------------------------------------- |
| `dir`     | Directory to search for templates                         |
| `glob`    | Pattern used to find the templates. Default is `"*.y*ml"` |


#### `chart.ytt(*files)`

Renders ytt templates using the `ytt` binary and returns a `stream`.

| Parameter | Description                                                                                                |
| --------- | ---------------------------------------------------------------------------------------------------------- |
| `files`   | These files are passed as `-f` option to `ytt`. You can also pass `stream`s returned from `helm` |

To access `self`, you need to use [`inject`](#inject) to inject this variable into your ytt files.


#### `chart.load_yaml(name)`

Load values from yaml file inside chart. The loaded values will override the existing values in self.

| Parameter | Description       |
| --------- | ----------------- |
| `name`    | Name of yaml file |

#### Attributes

| Name        | Description                                           |
| ----------- | ----------------------------------------------------- |
| `name`      | Name of the chart. Defaults to `self.__class__.name`  |
| `namespace` | Default namespace of the chart given via command line |
| `__class__` | Class of the chart. See `chart_class` for details     |


### Helm Charts

#### `helm_chart("<url>",namespace=namespace ,...)`

This load a helm chart, which will be installed using `helm upgrade -i`.
This is necessary, ti the helm chart uses hooks for installations. Otherwise you can directly use `chart`


### Dependencies

#### `depends_on("<url>", "constraint", namespace=namespace)`

This load a helm chart, which will be installed using `helm upgrade -i`.
This is necessary, ti the helm chart uses hooks for installations. Otherwise you can directly use `chart`


| Parameter   | Description                                                                                                                                                                                                                                  |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `url`       | The chart is loaded from the given url. The url can be relative.  In this case the chart is loaded from a path relative to the current chart location.                                                                                       |
| `constraint` | Version constraint for this chart, if it's already installed.                                                                                                                                                                  |
| `namespace` | If no namespace is given, the namespace is inherited from the parent chart.                                                                                                                                                                  |

It's also possible to configure dependencies like charts. In the future it will be possible check if properties
are changed in a compatible way.

### K8s


#### `k8s.delete(kind,name,namespaced=false,timeout=0,namespace=None,ignore_not_found=False)`

Deletes one kubernetes object

| Parameter          | Description                                                                                                               |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| `kind`             | k8s kind                                                                                                                  |
| `name`             | name of k8s object                                                                                                        |
| `timeout`          | Timeout passed to `kubectl apply`. A timeout of zero means wait forever.                                                  |
| `namespaced`       | If true object in the current namespace are deleted. Otherwise object in cluster scope will be deleted. Default is `true` |
| `namespace`        | Override default namespace of chart                                                                                       |
| `ignore_not_found` | Ignore not found                                                                                                          |

#### `k8s.apply(stream_or_object,namespaced=false,timeout=0,namespace=None,ignore_not_found=False)`

Deletes one kubernetes object

| Parameter          | Description                                                                                                               |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| `stream_or_object` | Can be a stream returned from `chart.template` or and `object` returned from `k8s.get`                                    |
| `timeout`          | Timeout passed to `kubectl apply`. A timeout of zero means wait forever.                                                  |
| `namespaced`       | If true object in the current namespace are deleted. Otherwise object in cluster scope will be deleted. Default is `true` |
| `namespace`        | Override default namespace of chart                                                                                       |
| `ignore_not_found` | Ignore not found                                                                                                          |

#### `k8s.get(kind,name,namespaced=false,timeout=0,namespace=None,ignore_not_found=False)`

Get one kubernetes object. The value is returned as a `dict`.

| Parameter          | Description                                                                                                             |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------- |
| `kind`             | k8s kind                                                                                                                |
| `name`             | name of k8s object                                                                                                      |
| `timeout`          | Timeout passed to `kubectl get`. A timeout of zero means wait forever.                                                  |
| `namespaced`       | If true object in the current namespace are listed. Otherwise object in cluster scope will be listed. Default is `true` |
| `namespace`        | Override default namespace of chart                                                                                     |
| `ignore_not_found` | Ignore not found                                                                                                        |

#### `k8s.patch(kind,name,patch,type='json',namespaced=false,timeout=0,namespace=None,ignore_not_found=False)`

Get one kubernetes object. The value is returned as a `dict`.

| Parameter          | Description                                                                                                             |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------- |
| `kind`             | k8s kind                                                                                                                |
| `name`             | name of k8s object                                                                                                      |
| `patch`            | patch, which should be applied                                                                                          |
| `type`             | Type of the patch. Currently on `json` is supported                                                                     |
| `timeout`          | Timeout passed to `kubectl get`. A timeout of zero means wait forever.                                                  |
| `namespaced`       | If true object in the current namespace are listed. Otherwise object in cluster scope will be listed. Default is `true` |
| `namespace`        | Override default namespace of chart                                                                                     |
| `ignore_not_found` | Ignore not found                                                                                                        |

#### `k8s.list(kind,namespaced=false,timeout=0,namespace=None,ignore_not_found=False)`

Get list of kubernetes object. The value is returned as a `dict`.

| Parameter          | Description                                                                                                             |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------- |
| `kind`             | k8s kind                                                                                                                |
  |
| `timeout`          | Timeout passed to `kubectl get`. A timeout of zero means wait forever.                                                  |
| `namespaced`       | If true object in the current namespace are listed. Otherwise object in cluster scope will be listed. Default is `true` |
| `namespace`        | Override default namespace of chart                                                                                     |
| `ignore_not_found` | Ignore not found                                                                                                        |

#### `k8s.watch(kind,name,namespaced=false,timeout=0,namespace=None,ignore_not_found=False)`

Watch one kubernetes object. The value is returned as a `iterator`.

| Parameter          | Description                                                                                                             |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------- |
| `kind`             | k8s kind                                                                                                                |
| `name`             | name of k8s object                                                                                                      |
| `timeout`          | Timeout passed to `kubectl watch`. A timeout of zero means wait forever.                                                |
| `namespaced`       | If true object in the current namespace are listed. Otherwise object in cluster scope will be listed. Default is `true` |
| `namespace`        | Override default namespace of chart                                                                                     |
| `ignore_not_found` | Ignore not found                                                                                                        |

#### `k8s.rollout_status(kind,name,timeout=0,namespace=None,ignore_not_found=False)`

Wait for rollout status of one kubernetes object

| Parameter          | Description                                                              |
| ------------------ | ------------------------------------------------------------------------ |
| `kind`             | k8s kind                                                                 |
| `name`             | name of k8s object                                                       |
| `timeout`          | Timeout passed to `kubectl apply`. A timeout of zero means wait forever. |
| `namespace`        | Override default namespace of chart                                      |
| `ignore_not_found` | Ignore not found                                                         |

#### `k8s.wait(kind,name,condition, timeout=0,namespace=None,ignore_not_found=False)`

Wait for condition of one kubernetes object

| Parameter          | Description                                                              |
| ------------------ | ------------------------------------------------------------------------ |
| `kind`             | k8s kind                                                                 |
| `name`             | name of k8s object                                                       |
| `condition`        | condition                                                                |
| `timeout`          | Timeout passed to `kubectl apply`. A timeout of zero means wait forever. |
| `namespace`        | Override default namespace of chart                                      |
| `ignore_not_found` | Ignore not found                                                         |

#### `k8s.for_config(kube_config_content)`

Create a new k8s object for a different k8s cluster

| Parameter             | Description            |
| --------------------- | ---------------------- |
| `kube_config_content` | Content of kube config |

#### `k8s.progress(value)`

Report progress of installation

| Parameter | Description               |
| --------- | ------------------------- |
| `value`   | A value between 0 and 100 |

#### Attributes

| Name   | Description                                                                                             |
| ------ | ------------------------------------------------------------------------------------------------------- |
| `host` | Name of the host where the kubernetes API server is running                                             |
| `tool` | Tool which is used for deployment. Possible values `kapp` or `kubectl`. This value can also be modified |


### user_credential

#### `user_credential(name,username='',password='',username_key='username',password_key='password')`

Creates a new user credential. All user credentials assigned to a root attribute inside a chart are automatically applied to kubernetes.

| Parameter      | Description                                                                                |
| -------------- | ------------------------------------------------------------------------------------------ |
| `name`         | The name of the kubernetes secret used to hold the information                             |
| `username`     | Username. If it's empty it's either read from the secret or created with a random content. |
| `password`     | Password. If it's empty it's either read from the secret or created with a random content. |
| `username_key` | The name of the key used to store the username inside the secret                           |
| `password_key` | The name of the key used to store the password inside the secret                           |

### properties

#### `property(type='string',default=None)`

Creates a new property.

| Parameter      | Description                                                                                |
| -------------- | ------------------------------------------------------------------------------------------ |
| `type`         | Type of the property                             |
| `default`     | Default value |


#### `struct_property(*kwargs)`

Creates a new structured property.

| Parameter      | Description                                                                                |
| -------------- | ------------------------------------------------------------------------------------------ |
| `kwargs`       | List of proerties (e.g. `password = property()`)  |

#### `chart_property()`

Creates a property to hold a reference to another chart.

##### Attributes

| Name       | Description                                                                                                                          |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `username` | Returns the content of the username attribute. It is only valid after calling `chart.__apply(k8s)` or it was set in the constructor. |
| `password` | Returns the content of the password attribute. It is only valid after calling `chart.__apply(k8s)` or it was set in the constructor. |

### certificate

#### `certificate(name,ca_key='ca.crt',private_key_key='tls.key',cert_key='tls.crt',is_ca=false,signer=None,domains=[],validity='P3M')`

Creates a new certificate. All certificates assigned to a root attribute inside a chart are automatically applied to kubernetes.

| Parameter         | Description                                                    |
| ----------------- | -------------------------------------------------------------- |
| `name`            | The name of the kubernetes secret used to hold the information |
| `ca_key`          | The key which is used to store the CA into the secret          |
| `private_key_key` | The key which is used to store the private key into the secret |
| `cert_key`        | The key which is used to store the certificate into the secret |
| `is_ca`           |                                                                |
| `signer`          | The signing certificate                                        |
| `validity`        | The period if validity in ISO-8601 format                      |
| `domains`         | The list of DNS names                                          |

### config_value

#### `config_value(name,type='string',default='',description='Long description',options=[])`

Creates a config value. The user is asked for the value. 

| Parameter     | Description                                                                                       |
| ------------- | ------------------------------------------------------------------------------------------------- |
| `name`        | The name of the kubernetes secret used to hold the information. Also the name of the config value |
| `type`        | Can be `string`,`password`,`bool`,`selection`. Default if `string`                                |
| `default`     | Default value                                                                                     |
| `description` | A description                                                                                     |
| `options`     | Options. Only valid for type `selection`                                                          |

##### Attributes

| Name    | Description                 |
| ------- | --------------------------- |
| `value` | The value given by the user |

### struct

See [bazel documentation](https://docs.bazel.build/versions/master/skylark/lib/struct.html). `to_proto` and `to_json` are not yet supported.

### chart_class

The `chart_class` represents the values read from the `Chart.yaml` file

### stream

The `stream` class represents the values returned from `template`, `helm`, or `ytt` methods. Streams have not methods.
They can be passed to other templating functions. You can use `str` to convert them to strings

```python
self.config=str(self.ytt("template-file"))
```

### inject

This method can be used to pass additional parameters to ytt.

```python
inject(file1,file2,key1=value1,key2=value2)
```

| Name    | Description                 |
| ------- | --------------------------- |
| `file1...filen` | List of files or directories which should be used for injection |
| `key1=value1` | Values, which should be injected into the files |


#### Attributes

| Name          | Description |
| ------------- | ----------- |
| `api_version` | API version |
| `name`        | Name        |
| `version`     | Version     |
| `description` | Description |
| `keywords`    | Keywords    |
| `home`        | Home        |
| `sources`     | Sources     |
| `icon`        | Icon        |

### utility variables

| Name           | Description        |
| -------------- | ------------------ |
| `version`      | shalm version      |
| `kube_version` | Kubernetes version |


## Libraries

The following libraries are available through the [`load` statement](https://github.com/google/starlark-go/blob/master/doc/spec.md#load-statements)

| Name          | Description |
| ------------- | ----------- |
| `@ytt:base64` |  See [ytt documentation](https://github.com/k14s/ytt/blob/master/docs/lang-ref-ytt.md)           |
| `@ytt:json`   |  See [ytt documentation](https://github.com/k14s/ytt/blob/master/docs/lang-ref-ytt.md)           |
| `@ytt:md5`    |  See [ytt documentation](https://github.com/k14s/ytt/blob/master/docs/lang-ref-ytt.md)           |
| `@ytt:regexp` |  See [ytt documentation](https://github.com/k14s/ytt/blob/master/docs/lang-ref-ytt.md)           |
| `@ytt:sha256` |  See [ytt documentation](https://github.com/k14s/ytt/blob/master/docs/lang-ref-ytt.md)           |
| `@ytt:url`    |  See [ytt documentation](https://github.com/k14s/ytt/blob/master/docs/lang-ref-ytt.md)           |
| `@ytt:yaml`   |  See [ytt documentation](https://github.com/k14s/ytt/blob/master/docs/lang-ref-ytt.md)           |
| `@shalm:bcrypt` | Method to create password hashes using bcrypt          |
| `@shalm:osb`   | Open Service Broker Bindings See below           |


### Open Service Broker Bindings