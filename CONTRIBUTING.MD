# Contributing to Wireflow

## 🎉 Welcome!

Thank you for considering contributing to Wireflow!

## 🚀 Quick Start

### Prerequisites
- Go 1.24+
- Kubernetes cluster (k3s/kind/minikube)
- Docker

### Setup Development Environment

```bash
# Clone the repo
git clone https://github.com/wireflowio/wireflow.git
cd wireflow

# Install dependencies
make install-deps

# Run tests
make test

# Run locally
make run
```

### How to Contribute
1. Reporting Bugs
   Use our bug report template
2. Suggesting Features
   Use our feature request template
3. Contributing Code
   - Fork the repository
   - Create a feature branch: git checkout -b feature/amazing-feature
   - Make your changes
   - Run tests: make test
   - Commit: git commit -m 'Add amazing feature'
   - Push: git push origin feature/amazing-feature
   - Open a Pull Request
### Code Style
   - Follow Effective Go
   - Run make lint before committing
   - Write tests for new features
### Good First Issues
   Looking for where to start? Check issues labeled good first issue
   
### Questions?
   Open a GitHub Discussion
   Join our Slack 
### Code of Conduct
   Be respectful and inclusive. See CODE_OF_CONDUCT.md