## Using environments

You can switch `environment` (or context) in which a flow is executed by running `var env set <env name>`.

```
$ var env set dev
$ var test
#=> reads inputs from var.yaml + config/environments/dev.yaml

$ var env set prod
$ var test
#=> reads inputs from var.yaml + config/environments/prod.yaml
```
