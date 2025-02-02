# Installing Lagoon Into Existing Kubernetes Cluster

## Requirements&#x20;

* Kubernetes 1.19+ (Kubernetes 1.22+ is not yet supported, see https://github.com/uselagoon/lagoon/issues/2816 for progress)
* Familiarity with [Helm](https://helm.sh) and [Helm Charts](https://helm.sh/docs/topics/charts/#helm), and [kubectl](https://kubernetes.io/docs/tasks/tools/).
* Ingress controller (ingress-nginx)
* Cert manager (for TLS) - We highly recommend using letsencrypt
* RWO storage

!!! Note "Note:"
    We acknowledge that this is a lot of steps, and our roadmap for the immediate future includes reducing the number of steps in this process.&#x20;
