# CI/CD Cluster Setup Guide

This guide provides detailed instructions to set up a GitLab Runner CI/CD cluster using **k3s**, **Flux CD**, and **SOPS** for secret management. The setup is designed for an air-gapped environment, using private Docker registries and Helm chart repositories.

## Components Overview

### k3s

k3s is a lightweight Kubernetes distribution optimized for resource efficiency, making it ideal for production and development clusters, particularly in edge or air-gapped environments.

### Flux CD

Flux CD automates the deployment and management of Kubernetes resources from a Git repository, applying a GitOps workflow. It monitors Git repositories and automatically synchronizes changes to the Kubernetes cluster.

### SOPS

SOPS (Secrets Operations) is used to securely manage and encrypt sensitive data in your repository, such as passwords, tokens, and certificates. SOPS works seamlessly with Flux, enabling encrypted secrets to be decrypted automatically in the cluster.

### GitLab Runner

GitLab Runner is a component that handles running CI/CD jobs defined in GitLab. In this setup, the GitLab Runner uses the Kubernetes executor to dynamically scale build and CI jobs. Jobs are executed within Docker containers specified by your CI/CD pipeline configuration, providing isolated and reproducible environments.

### GitLab Runner Helper

The GitLab Runner Helper is a specialized Docker image used by the Kubernetes executor alongside job-specific images. It manages cloning repositories, caching, and uploading artifacts, streamlining the build and deployment processes.

## Prerequisites and Requirements

### Root Access to SDS-GITLABSP-01

Ensure you have root access on the `SDS-GITLABSP-01` host. Execute the following command to gain root privileges:

```shell
sudo -i
```

### Helm and Docker Images

This CI/CD setup relies on the following ProGet feeds:

- **Feedgroup:** `ais`
  - **Docker feed:** `ais-docker-hub`
  - **Helm feed:** `k3s-tooling`

You will need a ProGet API key, this project is using the key named `ais-docker-download`.

Most container images within the `ais-docker-hub` feed are automatically retrieved from Docker Hub. However, you must manually add the GitLab Runner Helm Chart from [ArtifactHub](https://artifacthub.io/packages/helm/gitlab/gitlab-runner) into the `k3s-tooling` Helm feed.

> **Note:** An improvement to this setup could include configuring an ArtifactHub connector within the `k3s-tooling` feed. This enhancement has not yet been implemented at the time of writing.

## K3s Installation (Air-gapped)

[Official Documentation](https://docs.k3s.io/installation/airgap)

### 1. Download Dependencies

The following dependencies are needed for the air-gapped installation:

- **k3s airgap image**: Includes required container images for the Kubernetes system.
- **k3s binary**: Kubernetes runtime executable.
- **Install script**: Automates k3s setup.
- **SELinux RPM**: Handles security permissions and policies required by k3s.

Run:

```shell
mkdir -p /var/lib/rancher/k3s/agent/images/
curl https://pb.sykehuspartner.no/endpoints/app/content/k3s/k3s-airgap-images-amd64.tar.gz \
    --output "/var/lib/rancher/k3s/agent/images/k3s-airgap-images-amd64.tar.gz"

curl https://pb.sykehuspartner.no/endpoints/app/content/k3s/k3s \
    --output "/usr/local/bin/k3s" && chmod +x /usr/local/bin/k3s

curl https://pb.sykehuspartner.no/endpoints/app/content/k3s/install.sh \
    --output "~/install.sh" && chmod +x ~/install.sh

curl https://pb.sykehuspartner.no/endpoints/app/content/k3s/k3s-selinux-1.4-1.el9.noarch.rpm \
    --output "~/k3s-selinux-1.4-1.el9.noarch.rpm"

yum install ~/k3s-selinux-1.4-1.el9.noarch.rpm
```

### 2. Install k3s

```shell
INSTALL_K3S_SKIP_DOWNLOAD=true ~/install.sh
```

### 3. Configure Kubernetes Environment

Set KUBECONFIG environment globally to manage Kubernetes from any session:

```shell
echo "export KUBECONFIG=/etc/rancher/k3s/k3s.yaml" >> /etc/environment
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
```

### 4. Private Docker Registry Configuration

k3s uses this registry to pull container images privately:

Create `/etc/rancher/k3s/registries.yaml`:

```yaml
mirrors:
  docker.io:
    endpoint:
      - https://pb.sykehuspartner.no/ais-docker-hub/
configs:
  docker.io:
    auth:
      username: api
      password: <ais-docker-download API-key>
```

## Flux CD Setup

### 1. Install Flux CLI

The Flux CLI manages the Flux lifecycle from the command line:

```shell
curl https://pb.sykehuspartner.no/endpoints/app/content/k3s/flux \
    --output "/usr/local/bin/flux" && chmod +x /usr/local/bin/flux
```

### 2. GitLab Access Token

Navigate to your GitLab repository containing the CI/CD project. Go to `Settings` → `Access Tokens` → `Add New Token` and create a token with the following details:

- **Token Name:** `flux-auth-build-cicd-cluster`
- **Role:** Owner
- **Scopes:** `api`, `read_api`, `read_repository`, `write_repository`

Copy the generated token for use in the next step.

### 3. Bootstrap Flux

Flux initializes itself using a Git repository, managing synchronization and applying manifests.

Replace the `<PROJECT_API_TOKEN>` and `<ais-docker-download API-key>` values, and run:

```shell
export GITLAB_TOKEN="<PROJECT_API_TOKEN>"

flux bootstrap gitlab \
  --token-auth \
  --hostname gitlab.sikt.sykehuspartner.no \
  --owner=gitlab \
  --repository=build-cluster \
  --registry pb.sykehuspartner.no/ais-docker-hub/fluxcd \
  --registry-creds=api:<ais-docker-download API-key> \
  --image-pull-secret=regcred \
  --branch=main \
  --path=clusters/sds-gitlabsp-01 \
  --insecure-skip-tls-verify \
  --ca-file /etc/pki/ca-trust/source/anchors/HSO-ROOT-CA.cer
```

## SOPS and Age Secret Management

- [SOPS Documentation](https://github.com/getsops/sops)
- [age Documentation](https://github.com/FiloSottile/age)

### Server-side Installation

Install SOPS and age:

```shell
curl https://pb.sykehuspartner.no/endpoints/app/content/SOPS/sops-v3.9.3.linux.amd64 --output "/usr/local/bin/sops" && chmod +x /usr/local/bin/sops

curl -L https://pb.sykehuspartner.no/endpoints/app/content/SOPS/age-v1.2.1-linux-amd64.tar.gz | \
    tar -xz --no-same-owner -C /usr/local/bin --strip-components=1 age/age age/age-keygen && chmod +x /usr/local/bin/age /usr/local/bin/age-keygen
```

Ensure `/usr/local/bin` is in your PATH by adding:

```shell
echo 'export PATH="$PATH:/usr/local/bin"' >> ~/.bashrc
source ~/.bashrc
```

### Client-side Installation (Windows)

By installing the SOPS application to a Windows Admin host you will be able to encrypt secrets locally.

Download [SOPS Windows binary](https://pb.sykehuspartner.no/endpoints/app/content/SOPS/sops-v3.9.4.exe) and place it at:

```text
C:\Program Files (x86)\Sops\sops.exe
```

Update your system's PATH:

```powershell
$oldPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
$newPath = "$oldPath;C:\Program Files (x86)\Sops"
[Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
```

### Generate Encryption Key (Age)

Create a new encryption key on your cluster:

```shell
age-keygen -o age.agekey

# Output:
# Public key: age1he...fwq98cmsg
```

Store the private key securely and add it to Kubernetes:

```shell
cat age.agekey | kubectl create secret generic sops-age \
  --namespace=flux-system \
  --from-file=age.agekey=/dev/stdin
```

### Configure `.sops.yaml`

Create a `.sops.yaml` file in `/app/gitlab-runner/` directory containing:

```yaml
creation_rules:
  - encrypted_regex: "^(data|stringData)$"
    age: <Public Key> # Example: age1he...fwq98cmsg
```

You can now encrypt `data` and `stringData` fields in YAML files with this simple command:

```powershell
sops encrypt --in-place .\example-file.yaml
```

> Note: You will need to be in the same directory as the `.sops.yaml` file.

## GitLab Runner Secrets Setup

These steps will prepare credentials and certificates used by the GitLab-Runner application.
We will start by adding secrets in the the `app/gitlab-runner/secrets.yaml` file, then encrypt it using SOPS.

> Improvements: Currently this project stores most of the secrets inside a single file, this has proven to be painful when adding or editing a single entry. In the future we recommend using one file per secret.

### Docker Registry

Login and base64-encode Docker credentials:

```shell
docker login pb.sykehuspartner.no
base64 -w 0 ~/.docker/config.json && echo
```

Paste encoded credentials into `app/gitlab-runner/secrets.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: docker-registry-credentials
  namespace: gitlab-runner
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: <base64-encoded-json>
```

### Helm Chart Credentials

Insert ProGet API key:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: proget-helm-secret
  namespace: gitlab-runner
stringData:
  username: api
  password: <ais-docker-download API-key>
```

### TLS Certificates (ProGet and GitLab)

Fetch certificates and encode:

```shell
openssl s_client -showcerts \
    -connect pb.sykehuspartner.no:443 < /dev/null 2>/dev/null | \
    openssl x509 -outform PEM | base64 -w 0 && echo

openssl s_client -showcerts \
    -connect gitlab.sikt.sykehuspartner.no:443 < /dev/null 2>/dev/null | \
    openssl x509 -outform PEM | base64 -w 0 && echo
```

Insert encoded certs into `secrets.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: tls-crt-pb
  namespace: gitlab-runner
type: Opaque
data:
  ca.crt: <ProGet-cert>

---
apiVersion: v1
kind: Secret
metadata:
  name: tls-crt-gitlab
  namespace: gitlab-runner
data:
  gitlab.sikt.sykehuspartner.no.crt: <GitLab-cert>
```

### GitLab Registration Token

Navigate to `Settings -> CI/CD -> Runners -> New Project Runner` in GitLab, configure details, and copy the token:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gitlab-runner-token
  namespace: gitlab-runner
type: Opaque
stringData:
  runnerRegistrationToken: <token>
```

### Encrypt Secrets

Encrypt your secrets file:

```powershell
cd .\app\gitlab-runner
sops encrypt --in-place .\secrets.yaml
```

:warning: **WARNING!** :warning:

**Ensure the `secrets.yaml` file is encrypted before commiting**

```yml
# Unencrypted - Do not commit!
stringData:
    username: superman
    password: Clark_Kent123!

# Encrypted - Safe to commit.
stringData:
    username: ENC[AES256_GCM,data:Ud...UyxTJkw==,type:str]
    password: ENC[AES256_GCM,data:HrQT...MX0hU8fwKxQ==,type:str]
```

## Flux and SOPS Integration

Configure Flux to automatically decrypt secrets:

Create `gitlab-runner.yaml` in `./clusters/sds-gitlabsp-01/`:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: gitlab-runner
  namespace: flux-system
spec:
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
  path: ./app/gitlab-runner
  targetNamespace: gitlab-runner
  prune: true
  decryption:
    provider: sops
    secretRef:
      name: sops-age
```