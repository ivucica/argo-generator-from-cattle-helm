Starting point:

> Write a fully functioning ArgoCD generator plugin (including the Dockerfile or equivalent to create the actual image either in Docker format or OCI format, and the command line to generate it) that can turn helm.cattle.io/v1 object HelmChart into the Argo equivalent (picking the chart and its values from spec.chart, spec.version, spec.repo, spec.valuesContent). There must be the minimal source code for the plugin, preferably in Go and built statically as simply and quickly as possible, with minimal imports (no matter the chosen language -- the goal is single binary, tiny OCI / Dockerfile with fewest build steps, fewest dependencies, while still turning that YAML into different output understood by ArgoCD generator). Include detailed instructions on how to configure ArgoCD to use this new image. Invent the image name.

Instead of using `quay.io/my-org/argocd-helmchart-generator:v1.0.0`, removed hostname for now and using `argo-generator-from-cattle-helm:v1.0.0`.

Renamed binary from `helmchart-generator` to `argo-generator-from-cattle-helm`.

`kubectl patch deployment argocd-repo-server -n argocd --patch-file argocd-repo-server-patch.yaml` might work, but ideally we'd apply using our argo app + helmchart.

`kubectl edit -n argocd argocd-cm` needs to add the following data:

```
  applicationset.generators.plugins: |
    - name: helmchart-generator
      command:
        - helmchart-generator
```

An applicationset will need to use it then:

```
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: my-helm-charts
  namespace: argocd
spec:
  generators:
  - plugins:
      - name: helmchart-generator
        input:
          # This path will be passed to your plugin
          path: "path/to/your/helmchart/definitions"
        repo:
          # The source repo where the HelmChart objects are defined
          url: "https://github.com/your-org/your-git-repo.git"
          # The revision of the repo
          revision: "HEAD"
  template:
    metadata:
      # The name of the generated ArgoCD Application
      name: '{{name}}'
      # The namespace where the application will be created
      namespace: argocd
    spec:
      project: "default"
      source:
        repoURL: '{{repoURL}}'
        chart: '{{chart}}'
        targetRevision: '{{version}}'
        helm:
          values: '{{values}}'
      destination:
        # Deploy the chart to the same namespace as the HelmChart object's metadata
        namespace: '{{namespace}}'
        server: "https://kubernetes.default.svc"
```
