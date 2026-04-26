# talos-backup

talos-backup is a dead simple backup tool for Talos Linux-based Kubernetes clusters.
The goal is simple: run this as a cronjob in a desire cluster, take an etcd snapshot, push said snapshot to s3.

## Installation

talos-backup runs directly in Kubernetes on a given Talos cluster.

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
Run `age-keygen` and backup the keys in a place where you won't lose them.

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

## Configuration

### Compression

About compression, it is disabled by default.
You can turn it on by setting ENABLE_COMPRESSION to "true" in the environement variable list in `cronjob.sample.yaml`.
Talos backup will compress the etcd snapshot with zstd algorithm before encrypt it.

### Use Path Style

For local or self-hosted S3-compatible providers that require path-style addressing, set `USE_PATH_STYLE` to `"true"`.
When `USE_PATH_STYLE` is unset or `"false"`, talos-backup keeps the default minio-go bucket lookup behavior.

### Retention

The easiest way to set retention is to set the lifecycle policy on the storage bucket itself.

#### S3 backend specific notes

* Backblaze:
  * In the list of buckets, click on "Lifecycle Settings" next to your bucket that stores the Talos backups.
  * Select "Use custom lifecycle rules".
  * Leave the "File Path" box empty.
  * In the "Days Till Hide", put the number of days - 1 of your desired retention, then 1 on "Days Till Delete".
  * Click on "Update Bucket"

## Development

You may build the binary with:

```bash
make talos-backup
```

or as a container image with:

```bash
make REGISTRY=registry.example.com USERNAME=myusername PUSH=true TAG=latest image-talos-backup
```
