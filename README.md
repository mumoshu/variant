## Why?

Automating DevOps workflows is difficult because it often involve multiple `executables` like shell/ruby/perl/etc scripts and commands.

Because those executables vary in

* Its quality, from originally one-off script written in a day but living for several months or even years, to serious commands which are well-designed and written in richer programming languages with enough testing,
* Its interface. Passing parameters via environment variables, application specific command-line flags, configuration files

writing a single tool which wires up all the executables is time-consuming.

## What?

Variant is a framework to build a CLI application which becomes the single entry point to your DevOps workflows.

It consists of:

* YAML-based DSL
  * to define a CLI app's commands, inputs
  * which allows splitting commands into separate source files, decoupled from each others
* Ways to configure your apps written using Variant via:
  * defaults
  * environment variables
  * command-line parameters
  * application specific configuration files
  * environment specific configuration files
* DI container
  * to implicitly inject required inputs to a commands from configuration files or outputs from another commands
  * to explicit inject inputs to commands and its dependencies via command-line parameters

## Inputs

An input named `myinput` for the task `mytask` can be one of follows, in order of precedense:

* Value of the command-line option `--myinput`
* Value of the configuration variable `mytask.myinput`
  * from the environment specific config file: `config/environments/<environment name>.yaml`
  * from the common config file: `<command name>.yaml`(normally `var.yaml`)
* Output of the flow `myinput`

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

## Examples

Extract variant's version using jq:

```
$ var version --output json | jq -c -r 'select(.msg == "version") | .framework_version'
0.0.3-rc1
```

## Similar projects

* [tj/robo](https://github.com/tj/robo)

## Interesting Readings

* [How to write killer DevOps automation workflows](http://techbeacon.com/how-write-killer-devops-automation-workflows)
* [progrium/bashstyle: Let's do Bash right!](https://github.com/progrium/bashstyle)

## License

Apache License 2.0
