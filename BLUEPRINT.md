We'll write a crossplane grpc server in typescript.
It's called crossplane-skyhook.
It will rely on yarn berry, that is already installed and configured.
We will support inline typescript code in the Composition manifest.
When we'll receive a request from crossplane, we'll spawn a separated node process (using NODE_OPTIONS='--no-warnings --experimental-strip-types' to support typescript interpretation at runtime, it's a new node 22 feature). The separated code node process will be a ts code that will import the composition inline ts code. Before the server must write it in a temp file or transmit it to the spawned process in another way, with all the request data. The spawned process will return using stdout, and the server will return to crossplance via grpc.
We'll need testing (no jest) with a trivial case, a custom crd that will generate configmap.
