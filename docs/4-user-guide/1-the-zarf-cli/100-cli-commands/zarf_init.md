# zarf init

Deploys the gitops service or appliance cluster on a clean linux box

### Synopsis

Flags are only required if running via automation, otherwise the init command will prompt you for your configuration choices

```
zarf init [flags]
```

### Options

```
      --components string      Comma-separated list of components to install.  Adding this flag will skip the init prompts for which components to install
      --confirm                Confirm the install without prompting
  -h, --help                   help for init
      --nodeport string        Nodeport to access the Zarf container registry. Between [30000-32767]
      --secret string          Root secret value that is used to 'seed' other secrets
      --storage-class string   Describe the StorageClass to be used
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace
```

### SEE ALSO

* [zarf](./0-zarf.md)	 - Small tool to bundle dependencies with K3s for air-gapped deployments

###### Auto generated by spf13/cobra on 20-May-2022
