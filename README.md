# guestbook-workshop

A workshop loosely based on https://github.com/DirectXMan12/kubebuilder-workshops/tree/kubecon-eu-2019

There are two phases:

1. Running the guestbook app
    1. Wherein we will use our human operational prowess to modify the application
1. Build an operator for the guestbook app
    1. Wherein we will use our human operational intention, guided by our automated operational prowess, to modify the application

## Intended Audience

This README is written to be useful to a wide range of kubernetes users: from having never touched kubernetes before through fairly expert. It does assume some level of familiarity with the command line, containers, docker, and orchestration, though with sufficiently advanced copy and paste skills it may make some sense even without that background.

More critical is an understanding of the Go programming language: we'll be using Go to automate our application, and so that knowledge will be essential (see below for some pointers).

Finally, these instructions were written exclusively for and tested exclusively on macOS, though almost everything should translate smoothly to various flavors of linux.

### Introduction to Go

There are a number of resources available to get started with Go. I recommend:

- https://tour.golang.org (quick, interactive intro)
- https://gobyexample.com (more comprehensive intro, and helpful reference)
- https://www.miek.nl/go/ (free online book)

And there are a number of other resources listed on the golang wiki: https://github.com/golang/go/wiki/Learn

## Getting Started

The only hard prerequisite is [homebrew](https://brew.sh), but you'll have a better time if Docker is also installed and running in advance:

```
which docker || (brew cask install docker && open -a Docker)
```

And then we can install the tools we'll be using for this workshop:

```
brew install golang kubectl kustomize && brew link --overwrite kubernetes-cli # to clobber docker's kubectl
GO111MODULE="on" go get sigs.k8s.io/kind@v0.4.0
which kind || export PATH="$PATH:$(go env GOPATH)/bin"

# kubebuilder
os=$(go env GOOS)
arch=$(go env GOARCH)

curl -sL https://go.kubebuilder.io/dl/2.0.0-beta.0/${os}/${arch} | tar -xz -C /tmp/

export PATH=$PATH:/usr/local/kubebuilder/bin
[ -e /usr/local/kubebuilder ] && sudo mv /usr/local/kubebuilder{,.bak}
sudo mv /tmp/kubebuilder_2.0.0-beta.0_${os}_${arch} /usr/local/kubebuilder
```

If you already have these tools installed, please ensure you're using golang > 1.12, kubectl > 1.15, and especially kustomize > 3.1.

Now that we've got [`kind`](https://sigs.k8s.io/kind) installed, let's make a cluster:

```
kind create cluster --name workshop
export KUBECONFIG="$(kind get kubeconfig-path --name="workshop")"
```

and check that everything's set up correctly:

```
kubectl cluster-info
```

One last setup step will be to save ourselves a good deal of typing:

```
alias k=kubectl
```

# Phase 1

## Run the guestbook app

```
k create -f https://raw.githubusercontent.com/kubernetes/examples/011284134a724c0ce30f9fa4ec966bdbdefb843e/guestbook/all-in-one/guestbook-all-in-one.yaml
```

## Interact with the guestbook app

First, let's inspect our pods:

```
k get pods
```

You should see some output similar to:

```
NAME                            READY   STATUS              RESTARTS   AGE
frontend-678d98b8f7-6bvd4       0/1     ContainerCreating   0          3s
frontend-678d98b8f7-sdzhj       0/1     ContainerCreating   0          3s
frontend-678d98b8f7-xc54m       0/1     ContainerCreating   0          3s
redis-master-545d695785-gfxkx   0/1     ContainerCreating   0          4s
redis-slave-84548fdbc-5hql6     0/1     ContainerCreating   0          3s
redis-slave-84548fdbc-qxszs     0/1     ContainerCreating   0          3s
```

Hooray, we have containers for both our frontend app and its datastore (redis). Once those containers are running (try: `k get po -w`), we can interact with our local kubernetes cluster with a little help from kubectl:

```
k port-forward service/frontend 8080:80
```

and visit http://localhost:8080 to view the guestbook in all its glory.

### Detail: Metadata, Spec, and Status

Let's take a closer look at a Pod:

```
k get $(k get po -l app=guestbook -o name | head -n1) -o yaml
```

The top-level keys in the YAML document are:

```
apiVersion: v1
kind: Pod
metadata:
...
spec:
...
status:
...
```

Every kubernetes object has an `apiVersion` and `kind` that specify what type the object is, and `metadata` that every object shares. Almost every object also has a `spec` and a `status`: the former defines what should be, and the latter is the observed state of what is.

All kubernetes components are responsible for looking at some part of the desired intent written down in an object, idempotently ratcheting the actual state one step closer, and updating the status.

## Make a change to the guestbook app

### Task: Add an annotation to the frontend pod

To get comfortable using `kubectl`, let's try making a trivial change: adding an annotation to one of the frontend pods. Run the commands below. With the edit open, add an `annotations` key underneath `metadata`, and add a "my-key: my-value" pair underneath that. For more about the `ObjectMeta` schema, take a look at the docs link in the references below.

```
k edit $(k get po -l app=guestbook -o name | head -n1)
k describe po -l app=frontend
```

Notice that one of the pods now has `Annotations:   my-key: my-val`.

### Exercise: Change the number of frontend pods

Goal: `k get po -l app=guestbook` returns four pods.

Hint: What does `k get po -l app=guestbook -o name | head -n1` produce? Use `k get all` to see what other targets you might `k edit`.

### Optional Exercise: Change the pod's environment

Goal: `k exec $(k get po -l app=guestbook -o name | head -n1) env` contains a new environment variable.

## Cleanup

Use

```
k delete -f https://raw.githubusercontent.com/kubernetes/examples/011284134a724c0ce30f9fa4ec966bdbdefb843e/guestbook/all-in-one/guestbook-all-in-one.yaml
```

to delete all the resources that were created as part of the setup.

# Phase 2

In phase two, we'll write an operator that encompasses the guestbook app, and actuates some changes from above.


```
git clone https://github.com/sethp-nr/guestbook-workshop
cd guestbook-workshop
export GO111MODULE=on
go mod init guestbook-workshop
```

Next, we'll need kubebuilder to scaffold out an empty project (with no custom types):

```
kubebuilder init --domain example.com
```

Once that completes, we'll have a lot of project infrastructure like a Makefile, Dockerfile, kubernetes config directory, and a `main.go`. The directory structure should look something like this:

```
$ tree .
.
├── Dockerfile
├── Makefile
├── PROJECT
├── bin
│   └── manager
├── config
│   ├── certmanager
│   │   ├── certificate.yaml
│   │   ├── kustomization.yaml
│   │   └── kustomizeconfig.yaml
│   ...
├── go.mod
├── go.sum
├── hack
│   └── boilerplate.go.txt
└── main.go
```

This is everything we need to get started, but we don't yet have an API for users to request an instance of a guest book application from us.

In kubernetes, the suggested API for an operator is to use a Custom Resource Definition (CRD) as a way to introduce new nouns beyond the built-in `Pod`s and `Service`s and `Deployment`s. Let's generate one now:

```
kubebuilder create api --group webapp --kind GuestBook --version v1 --resource --controller
```

That command added five new files across two new directories:

```
$ tree api controllers
api
└── v1
    ├── groupversion_info.go
    ├── guestbook_types.go
    └── zz_generated.deepcopy.go
controllers
├── guestbook_controller.go
└── suite_test.go
```

The `api/v1` directory contains our new api type information (and some generated function definitions), and `controllers` is where we'll encode our operational expertiese around the guest book application.

There's also a `suite_test.go` in the `controllers` package. One of the main advantages of using kubernetes for operational software is that it's relatively easy to write meaningful tests, and `suite_test.go` sets up some common test infrastructure you'll be likely to want. This repository contains a `template/` directory that includes tests for what we're going to cover today, so let's copy those over now:

```
cp template/controllers/guestbook_controller_test.go controllers/
```

The template also contains a `.vscode` directory, accessible with `cp -R template/.vscode .`, as that tool is a common aesthetic in the Go community. If you're not accustomed to using .vscode, fear not: your favorite $EDITOR should work just fine for this workshop.

## Task: Getting a GuestBook resource

The first thing to do in our new controller is replace `// your logic here` with something a little more useful. The first obstacle is knowing what our Reconcile should do: whereas before a Pod or Deployment resource held the desired state, now we've created a GuestBook CRD to hold that information.

But our Reconcile function isn't handed a GuestBook resource, it's just handed a `reconcile.Request`. Looking at the [Godoc](https://godoc.org/sigs.k8s.io/controller-runtime/pkg/reconcile#Request) we can see that a reconcile request contains just one piece of information:

```
type Request struct {
    // NamespacedName is the name and namespace of the object to reconcile.
    types.NamespacedName
}
```

So we'll need to use the [`client.Client`](https://godoc.org/sigs.k8s.io/controller-runtime/pkg/client) embedded in our `GuestBookReconciler`:

```
// GuestBookReconciler reconciles a GuestBook object
type GuestBookReconciler struct {
	client.Client
	Log logr.Logger
}
```

With that, we can ask the API Server for the current state of the object. To do so,

1. Add `apierrors "k8s.io/apimachinery/pkg/api/errors"` to the `import` section
2. Let's change the `Reconcile` function in `controllers/guestbook_controller.go` to look like this:

```
func (r *GuestBookReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("guestbook", req.NamespacedName)

	var guestbook webappv1.GuestBook
	err := r.Get(ctx, req.NamespacedName, &guestbook)
	if err != nil {
        if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Successfully retrieved GuestBook")

	return ctrl.Result{}, nil
}
```

Now run the tests with `make test`, and check out our fancy new log statement. You should see something like this:

> 2019-07-30T14:15:37.480-0700	INFO	Successfully retrieved GuestBook	{"guestbook": "default/test-guestbook"}

### An Aside: Level vs edge triggered behavior

Why are we looking up the GuestBook? Why aren't we provided with the spec that we're supposed to be actuating against? One of the key ideas in kubernetes is "level-triggered" behavior: instead of trying to catch every change that happens in the system and react to it, the job of a controller is to reconcile the most up-to-date picture it has and take one step closer to the desired state. Not only is this much easier to reason about, but it gracefully handles failures and "catching up" by re-running the Reconcile.

In this case, our Reconcile is running against a queue of objects, and the object may have changed since it was put in that queue. So instead of putting in the full object that may actually be out-of-date by the time our code runs, we instead make sure we're looking at the current level of the system.

That "current level" is also why we're exiting early if the GuestBook is not found: if someone asked us for a GuestBook instance, but withdrew their request before we could get to it, then we don't need to react to it at all.

## Exercise: Create a basic Deployment

Goal: The `Reconcile` function should use the [client's `Create` method](https://godoc.org/sigs.k8s.io/controller-runtime/pkg/client#Client) to make a frontend deployment matching the GuestBook resource.

Hint 1: The specific YAML we want our code to match is here: https://github.com/kubernetes/examples/blob/011284134a724c0ce30f9fa4ec966bdbdefb843e/guestbook/frontend-deployment.yaml

Hint 2: Use `import appsv1 "k8s.io/api/apps/v1"` to get access to the `appsv1.Deployment` type, and try copying the fields into a variable of that type one at a time

Hint 3: Use `import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"` for things like `metav1.ObjectMeta{}` and `metav1.LabelSelector`

Hint 4: If you're having trouble with the `Resources`, try stack overflow: https://stackoverflow.com/a/53002166

Hint 5: The `controllers/guestbook_controller_test.go` file contains an example of defining a `appsv1.Deployment{}` resource for the guestbook app

Solution: See `_solutions/create-deployment/guestbook_controller.go`

## Optional Exercise: Use a per-GuestBook label selector

Goal: If all the deployments we create use the same label selector, then we'll only ever get one set of frontend pods. Can we change the label selector to work when we have two GuestBooks?

Hint: The provided tests have a suggestion on how to change the labels and selector

## Optional Exercise: Create a Service

Goal: In addition to a Deployment, `Reconcile` should also create the frontend Service. If you're using the provided tests, there is a "pending" test (`PIt`) for this behavior.

Hint: The YAML we want to match is: https://github.com/kubernetes/examples/blob/011284134a724c0ce30f9fa4ec966bdbdefb843e/guestbook/frontend-service.yaml

## Task: Specifying a Desired State

So far we have expressed our intention to the system that there should be a GuestBook app (the instance of the GuestBook resource), but we haven't made it very easy for users of our API to affect what that app looks like. A common desire is scaling the number of replicas backing an HTTP service, so let's implement that.

First, we need a place to write that down. Open up the `api/guestbook_types.go` file and add some detail to the `GuestBookSpec`

```
// GuestBookSpec defines the desired state of GuestBook
type GuestBookSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Frontend FrontendSpec `json:"frontend"`
}

type FrontendSpec struct {
	// +optional
	Replicas *int32 `json:"replicas,omitempty"` // If it's nil, we'll use a default
}
```

Let's also update our `config/samples/webapp_v1_guestbook.yaml` to match our new schema:

```
apiVersion: webapp.example.com/v1
kind: GuestBook
metadata:
  name: guestbook-sample
spec:
  frontend:
    replicas: 3
```

and to prove that we've successfully put the data in and taken the data out, let's change our controller's log to:

```
log.Info("Successfully retrieved Guestbook", "replicas", guestbook.Spec.Frontend.Replicas)
```

### Exercise: Use the spec'd replicas

Goal: The deployment should be created with the replicas from the Spec. If you're using the provided tests, you can check by uncommenting the line following the TODO and change the `FIt` to just `It`.

### Exercise: Update the Deployment

Goal: If we change the spec'd replicas, the deployment should be updated.

Hint: You'll need to `Get` the expected `Deployment` and, if it exists, `Update` it instead of calling `Create`

Solution: See `_solutions/update-deployment/guestbook_controller.go`

## Running against kind

To see our hard work running against a real environment, let's create all the pieces we need except the deployment:

```
cat goal/{redis*,frontend-service}.yaml | k create -f -
```

And then we'll tell kubernetes about our custom type, and run our controller pointed at our kind cluster:

```
make install
make run
```

In a different terminal, let's ask for a GuestBook instance:

```
k apply -f config/samples/
```

And now we stand back and watch as the deployment is created on our behalf. Finally, we can run

```
k port-forward service/frontend 8080:80
```

and visit http://localhost:8080 to view the newly automated guestbook in all its glory.

After you're all done, don't forget to clean up:

```
kind delete cluster --name=workshop
```

# Where to go next

We left out a lot that's really well covered in the Kubecon EU workshop linked at the top. That includes:

1. The CreateOrUpdate function, which is useful for keeping code simple for mutable resources like Deployments
1. Getting the GuestBook right: we can run a lot of independent frontends, but they'll all currently talk to the same redis backend
1. Status updates on our guestbook resource
1. Controller references for speeding up how quickly we observe status
1. Managing the Guestbook service, as well as all the Redis components
1. Making the GuestBook CRD easier to use by putting information right in the `kubectl get` output
1. And a lot more of the "why," especially in the slides

That same repo also has another branch for building a MongoDB operator, and the [Kubebuilder book](https://kubebuilder.io/) is largely a tutorial on how to use kubebuilder to implement a CronJob type in kubernetes.

# References

- The slides from the original workshop: https://storage.googleapis.com/kubebuilder-workshops/kubecon-eu-2019.pdf
- Controller runtime godoc: https://godoc.org/sigs.k8s.io/controller-runtime/
- Kubernetes API reference: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/
    - Deployments: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#deployment-v1-apps
    - Services: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#service-v1-core
- Kubernetes API Godoc
    - ObjectMeta (cross-object metadata): https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#ObjectMeta
    - Deployments:  https://godoc.org/k8s.io/api/apps/v1#Deployment
    - Services: https://godoc.org/k8s.io/api/core/v1#Service

For more detail on some kubernetes concepts:

- Kubernetes objects: https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/
- Level triggering: https://hackernoon.com/level-triggering-and-reconciliation-in-kubernetes-1f17fe30333d

