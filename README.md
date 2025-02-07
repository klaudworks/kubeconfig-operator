# Kubeconfig Operator

This controller implements a `Kubeconfig` custom resource to generate a kubeconfig file with a specified set of permissions.

```yaml
apiVersion: klaud.works/v1alpha1
kind: Kubeconfig
metadata:
  name: my-kubeconfig
spec:
  clusterName: local-kind-cluster
  server: https://127.0.0.1:52856 # specify external endpoint to your kubernetes API.
  clusterPermissions:
    rules:
    - apiGroups:
      - ""
      resources:
      - namespaces
      verbs:
      - get
      - list
      - watch
  namespacedPermissions:
  - namespace: default
    rules:
    - apiGroups:
      - ""
      resources:
      - configmaps
      verbs:
      - '*'
  - namespace: kube-system
    rules:
    - apiGroups:
      - ""
      resources:
      - configmaps
      verbs:
      - get
      - list
      - watch

```

## Installation

## Local Development


1. Clone the repository:

    ```
    git clone github.com:klaudworks/kubeconfig-operator.git
    ```

1. Ensure you install [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) or you have another Kubernetes distribution installed.

1. Open `manifests/base/manager.yaml` and replace `image: REPLACE-ME` with `image: achilles-token-controller:latest`.
   If this file doesn't exist, run `make generate`.
1. Create the namespace for the controller
   ```sh
   kubectl create namespace achilles-system
   ```
1. Deploy the controller.
    ```sh
    kubectl apply -f manifests/base/manager.yaml
    ```
1. Test the controller with this example AccessToken.
   ```yaml
   apiVersion: group.example.com/v1alpha1
   kind: AccessToken
   metadata:
     name: test
     namespace: default
   spec:
     namespacedPermissions:
     - namespace: default
       rules:
       - apiGroups: [""]
         resources: ["configmaps"]
         verbs:     ["*"]
     - namespace: kube-system
       rules:
       - apiGroups: [""]
         resources: ["configmaps"]
         verbs:     ["get", "list", "watch"]
     clusterPermissions:
       rules:
       - apiGroups: [""]
         resources: ["namespaces"]
         verbs:     ["get", "list", "watch"]
    ```
1. Check that the AccessToken was processed successfully
   ```sh
   kubectl get accesstoken test -n default -oyaml
   ```

   You should see the following status condition, indicating that the object was instantiated successfully.

   ```yaml
    status:
      conditions:
      - lastTransitionTime: "2024-10-24T17:33:35Z"
        message: All conditions successful.
        observedGeneration: 1
        reason: ConditionsSuccessful
        status: "True"
        type: Ready
    ```
   You'll also see that it provisioned a deploy token as a secret, whose name is under `status.tokenSecretRef`.

