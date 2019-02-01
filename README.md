# Variant

[![CircleCI](https://circleci.com/gh/mumoshu/variant.svg?style=svg)](https://circleci.com/gh/mumoshu/variant)

`Variant` is a task runner / CLI generator.

Write your workflows in YAML. Add a shebang to call `variant`. `Variant` transforms your workflows into a modern CLI application.

```yaml
#!/usr/bin/env variant
tasks:
  hello:
   parameters:
   - name: target
   script: |
     echo Hello {{ get "target" }}!
```

```console
$ ./mycli --target variant
Hello variant! 
```

![image](https://user-images.githubusercontent.com/22009/51234992-b1899380-19b1-11e9-83c3-dbfdb1517b1c.png)

# Rationale

Automating DevOps workflows is difficult because it often involve multiple `executables` like shell/ruby/perl/etc scripts and commands.

Bause those executables vary in:

* Its quality, from originally one-off script written in a day but living for several months or even years, to serious commands which are well-designed and written in richer programming languages with enough testing,
* Its interface. Passing parameters via environment variables, application specific command-line flags, configuration files

Writing a single tool which

* wires up all the executables
* re-implements all the things currently done in various tools

is both time-consuming.

# Getting Started

Download and install `variant` from the GitHub releases page:

https://github.com/mumoshu/variant/releases

Create a yaml file named `myfirstcmd` containing:

```yaml
#!/usr/bin/env variant

tasks:
  bar:
    script: |
      echo "dude"
  foo:
    parameters:
    - name: bar
      type: string
      description: "the bar"
    - name: environment
      type: string
      default: "heaven"
    script: |
      echo "Hello {{ get "bar" }} you are in the {{ get "environment" }}"
```

Now run your command by:

```console
$ chmod +x ./myfirstcmd
$ ./myfirstcmd
Usage:
  myfirstcmd [command]

Available Commands:
  bar
  env         Print currently selected environment
  foo
  help        Help about any command
  ls          test
  version     Print the version number of this command

Flags:
  -c, --config-file string   Path to config file
  -h, --help                 help for myfirstcmd
      --logtostderr          write log messages to stderr (default true)
  -o, --output string        Output format. One of: json|text|bunyan (default "text")
  -v, --verbose              verbose output

Use "myfirstcmd [command] --help" for more information about a command.
```

Each task in the `myfirstcmd` is given a sub-command. Run `myfistcmd foo` to run the task named `foo`:

```console
$ ./myfirstcmd foo
Hello dude you are in the heaven
```

Look at the substring `dude` contained in the output above. The value `dude` is coming from the the parameter `bar` of the task `foo`. As we didn't specify the value for the parameter, `variant` automatically run the task `bar` to fulfill it.

To confirm that it is the task `bar` who fulfilled the value `dude`, run it:

```console
$ ./myfirstcmd bar
INFO[0000] ≫ sh -c echo "dude"
dude
```

To specify the value, use the corresponding command-line flag automatically created and named after the parameter `bar`:

```console
$ ./myfirstcmd foo --bar=folk
Hello folk you are in the heaven
```

Alternatively, you can source the value from a YAML file.

Create `myfirstcmd.yaml` containing:

```yaml
foo:
  bar: variant
```

Now your task sources `variant` as the value for the parameter:

```console
$ ./myfirstcmd foo
Hello variant you are in the heaven
```

# How it works

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

# Features

- Task grouping
- Dependency injection

## Dependency injection

An input named `myinput` for the task `mytask` can be one of follows, in order of precedense:

* Value of the command-line option `--myinput`
* Value of the configuration variable `mytask.myinput`
  * from the environment specific config file: `config/environments/<environment name>.yaml`
  * from the common config file: `<command name>.yaml`(normally `var.yaml`)
* Output of the task `myinput`

## Environments

You can switch `environment` (or context) in which a task is executed by running `var env set <env name>`.

```
$ var env set dev
$ var test
#=> reads inputs from var.yaml + config/environments/dev.yaml

$ var env set prod
$ var test
#=> reads inputs from var.yaml + config/environments/prod.yaml
```

# Alternatives

* [tj/robo](https://github.com/tj/robo)
* [goeuro/myke](https://github.com/goeuro/myke)

# Interesting Readings

* [How to write killer DevOps automation workflows](http://techbeacon.com/how-write-killer-devops-automation-workflows)
* [progrium/bashstyle: Let's do Bash right!](https://github.com/progrium/bashstyle)
* [ralish/bash-script-template: A best practices Bash script template with many useful functions](https://github.com/ralish/bash-script-template)

# Future Goals

* Runners to run tasks in places other than the host running your Variant app
  * Docker
  * Kubernetes
  * etc
* Tools/instructions to package your Variant app for easier distribution
  * Single docker image containing
    * all the scripts written directly in the yaml
    * maybe all the scripts referenced from scripts in the yaml
    * maybe all the commands run via the host runner
* Integration with job queues
  * to ensure your tasks are run reliably, at-least-once, tolerating temporary failures

# License

Apache License 2.0


# Attribution

We use:

- [semtag](https://github.com/pnikosis/semtag) for automated semver tagging. I greatly appreciate the author(pnikosis)'s effort on creating it and their kindness to share it!
