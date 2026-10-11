# AWS: one EC2 node

The prod env (gitops `bootstrap/root.yaml`) on a single `m7i-flex.large` running k3s, in an AWS Free plan account paid from credits. No load balancer, NAT or Elastic IP; the app answers on the node's public IP over https with a self-signed certificate.

| Piece | Choice | Why |
|---|---|---|
| Node | `m7i-flex.large`: 2 vCPU, 8 GB, x86 | largest Free plan type; no CPU credits to run out of. The prod profile (search and events on, ClickHouse capped at 1.5Gi; obs and tools off) idles at ~0.3 core and ~5.6 GiB. Events stay on: ClickHouse is the feed's only durable source |
| Disk | 30 GB gp3, encrypted | OS, images and every PVC (k3s local-path) |
| Network | own VPC `10.1.0.0/16`, one public subnet | free; nothing shared with other stacks |
| Address | one auto-assigned public IPv4 | changes on stop/start, but bills nothing while stopped. Login redirects are relative and `dsn-tls` is remade for the new IP at boot, so nothing in git holds the IP |
| Ingress | k3s Traefik as the Gateway, servicelb on host 80/443 | no cloud controller runs in k3s, so a LoadBalancer Service never creates an AWS LB |
| Access | SSH from one /32; the Kubernetes API through an ssh tunnel | nothing but 80/443 is public |
| Guardrail | Budget `dsn-credit-burn`, $100/month gross (credits excluded), account-wide, actual + forecast alerts | net cost stays $0 on the Free plan, so only gross cost shows the burn |
| State and secrets | S3 bucket + SSM SecureString parameters | shared by every machine with the account's credentials; under $0.01/month (SSM standard parameters and the `aws/ssm` key are free) |

Cost, gross, from credits: ~$70/month compute + ~$3.65 IPv4 + ~$2.40 disk = ~$76/month running; ~$2.40/month stopped.

## Rules
- State the cost of any new AWS resource before adding it
- `terraform apply` only after reviewing the plan; `terraform destroy` wipes every volume
- Never join AWS Organizations, Control Tower or the Partner Network: each upgrades a Free plan account to paid
- The node keeps no data outside its root volume: replacing it (a new AMI, an edited `user_data.sh`) starts from empty, so both are in `ignore_changes`

## State and secrets
Shared through the account, so any machine with its AWS credentials can run everything here; nothing travels between machines but those credentials.

| What | Where |
|---|---|
| Terraform state | S3 bucket `dsn-tfstate-<account id>` (`state/`: versioned, encrypted, private), keys `node/` and `state/`; `use_lockfile` stops two applies at once |
| SSH key, `terraform.tfvars`, OpenBao unseal key, prod values | SSM Parameter Store SecureString `/dsn/ssh-key`, `/dsn/tfvars`, `/dsn/openbao-unseal-key`, `/dsn/prod-secrets` (`secrets.sh`) |
| `kubeconfig`, `known_hosts` | made per machine by `connect.sh` |

`secrets.sh push` creates missing parameters from local files, `pull` writes missing files from the parameters; a value that differs on both sides stops it (`FORCE=1` replaces). Never force the unseal key once OpenBao has data: only the old key opens it.

## Up
Needs `terraform` >= 1.10, the `aws` CLI with credentials for the account, and the app's images for amd64 (the Publish workflow builds both archs).

Once per account, the state bucket (its own state moves into it):
```sh
cd deploy/aws/state
printf 'terraform {\n  backend "local" {}\n}\n' > override.tf
terraform init && terraform apply -var aws_account_id=<account id>
rm override.tf && terraform init -migrate-state -backend-config="bucket=dsn-tfstate-<account id>"
```

The node:
```sh
cd deploy/aws
cp terraform.tfvars.example terraform.tfvars   # account id, alert email, your ip /32
terraform init -backend-config="bucket=dsn-tfstate-<account id>"
terraform plan -out=dsn.tfplan
terraform apply dsn.tfplan
./bootstrap.sh
```

`bootstrap.sh` is safe to rerun. It syncs the secrets with SSM (generating the unseal key and prod values only where none exist), installs k3s (pinned), a boot-time unit that writes `dsn-tls` for the current IP, OpenBao's unseal key and first-start values, Argo CD with `bootstrap/argocd-config.yaml`, and the root app; then writes `kubeconfig` (through an ssh tunnel) and drops OpenBao's seed once it has started. Argo CD brings the app up on `https://<public ip>` in ~15 minutes.

## Another machine
```sh
cd deploy/aws
terraform init -backend-config="bucket=dsn-tfstate-$(aws sts get-caller-identity --query Account --output text)"
./connect.sh
```
`connect.sh` pulls the secrets, finds the node's current IP, writes `kubeconfig` and opens the ssh tunnel. Port 22 is open to one address (`ssh_cidr`); from a new network it says how to move it.

## Day to day
```sh
./connect.sh                                                 # also after the IP changed
export KUBECONFIG=$PWD/kubeconfig
kubectl -n argocd get applications
kubectl -n argocd port-forward svc/argocd-server 9443:443   # admin / argocd-initial-admin-secret
```

Stop when idle, start when needed (the IP changes; `connect.sh` and the node's certificate follow it):
```sh
aws ec2 stop-instances --instance-ids $(terraform output -raw instance_id)
aws ec2 start-instances --instance-ids $(terraform output -raw instance_id)
```
