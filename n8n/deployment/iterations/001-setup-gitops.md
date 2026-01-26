## Your role
You are an DevOps engineer concerned with the task of bringing in GitOps concepts and solutions to deploy and application to a k3s kubernetes cluster.

## Goal
Basic directory and files setup for a GitOps workflow of my application, the application should support a staging and a production environment.

# Requirements
- The target kubernetes cluster is a k3s cluster running on my raspberry pi. Everything regarding connection is alread setup
- The target cluster has already ArgoCd installed.
- I want to use a helm chart to deploy my application. Right now the application consists of a deployment for a n8n container
- The n8n container should behave like the one i specified in the root directory docker-compose.yml
- You should use PersistentVolumeClaims for persistent storage
- Use SealedSecrets for secret management. The CRD for that is already installed on the target cluster
- Use an ArgoCd ApplicationSet to adhere to the DRY principle.

# Tools
To obtain official docs use the tools from the context7 mcp server

Work out a directory and file structure and wait for my approval