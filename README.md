# talos-backup

talos-backup is a dead simple backup tool for Talos Linux-based Kubernetes clusters.
The goal is simple: connect to the cluster using the Talos API, take an etcd snapshot, push said snapshot to s3.

This is accomplished by properly configuring talos-backup with a config file.
Config files should look like:

```bash
snapshots:
  - clusterName: testy
    talosInfo:
      secret: "xxx"
      ## TODO: above needs to be an object ref to the talosconfig secret. currently just a b64 string
    s3Info:
      bucket: "one-testy-boi"
      region: "us-east-1"
    schedule: "*/2 * * * *"
    ## ^ a comically short every 2 mins.
  - clusterName: testy2
    talosInfo:
      secret: "xxx"
      ## TODO: above needs to be an object ref to the talosconfig secret. currently just a b64 string
    s3Info:
      bucket: "two-testy-boi"
      region: "us-east-1"
    schedule: "0 * * * *"
    ## ^ every hour on the hour
```

Each list item is a different cluster that should be backed up and can have differing s3 info as well as backup schedules.
For example, it may be far more important to backup prod on an hourly or daily basis, while staging gets weeklies.

## Future

Quick TODOs:

- make this k8s deployable

In the future this will morph a bit to be CAPI-aware, so that if this is run in a mgmt plane, it will immediately pickup any new clusters and try to back them up in some reasonable way.

It will also change to use a secret ref inside of k8s instead of a b64 encoded talosconfig.
This will make the above feature easier and remove gross in-line creds.
