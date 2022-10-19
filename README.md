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

## Backup Service

talos-backup may also run in a Talos cluster.
We have a program optimized for this purpose under `cmd/etcd-snapshot-k8s-service`.
You may build it as a binary with `make etcd-snapshot-k8s-service` or as a container image with

```bash
make REGISTRY=registry.example.com USERNAME=myusername PUSH=true TAG=latest image-etcd-snapshot-k8s-service
```

To enable the necessary Talos API access for a pod you will need the following modifications in your machine config:

```yaml
spec:
  machine:
    features:
      kubernetesTalosAPIAccess:
        enabled: true
        allowedRoles:
        - os:etcd:backup
        allowedKubernetesNamespaces:
        - default
```

You will need a public/private key pair to encrypt(public key) and decrypt(private key) your backups.
This service uses `age` for encryption.
Find [installation instructions here](https://github.com/FiloSottile/age#installation).
Run `age-keygen` and backup the keys in a place where you won't loose them.

The file `cronjob.sample.yaml` specifies a kubernetes CronJob that backs up a cluster every 10 minutes.
Customize it and substitute the age public key.
S3 configurations may be supplied in whatever way the Go AWS SDK v2 expects them, in this example we happen to use environment variables.

Apply the CronJob:

```bash
kubectl apply -f cronjob.sample.yaml
```

To test what you deployed you can trigger the job manually:

```bash
kubectl create job --from=cronjob/talos-backup my-test-job
```

## Future

Quick TODOs:

- make this k8s deployable

In the future this will morph a bit to be CAPI-aware, so that if this is run in a mgmt plane, it will immediately pickup any new clusters and try to back them up in some reasonable way.

It will also change to use a secret ref inside of k8s instead of a b64 encoded talosconfig.
This will make the above feature easier and remove gross in-line creds.
