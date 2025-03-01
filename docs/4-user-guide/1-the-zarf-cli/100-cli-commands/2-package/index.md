# zarf package

Pack and unpack updates for the Zarf gitops service.

### Options

```
  -h, --help   help for package
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace
```

### SEE ALSO

* [zarf](../0-zarf.md)	 - Small tool to bundle dependencies with K3s for air-gapped deployments
* [zarf package create](./zarf_package_create.md)	 - Create an update package to push to the gitops server (runs online)
* [zarf package deploy](./zarf_package_deploy.md)	 - Deploys an update package from a local file or URL (runs offline)
* [zarf package inspect](./zarf_package_inspect.md)	 - lists the payload of an update package file (runs offline)

###### Auto generated by spf13/cobra on 20-May-2022
