# croosplane-skyhook blueprint

We'll write a grpc server for crossplane functions. The grpc server must be written in go.
This server will create a deterministic hash from inline javascript/typescript source code from crossplane composition.
Then the server will create a nodejs subprocess (direct official node binary execution) if doesn't exists for the hash of the inline js/ts. Then the server will relay the request to the node subprocess, the node subprocess will run the inline javascript/typescript to return to the go cerver that will relay to crossplane via grpc.
The node part will rely on yarn berry, that is already installed and configured.
we'll use NODE_OPTIONS='--no-warnings --experimental-strip-types' to support typescript interpretation at runtime, it's a new node 22 feature.
The server must write the js/ts from inline it in a temp file or transmit it to the spawned process in another way, with all the request data.
We'll need e2e testing (no jest) with a trivial case, a custom crd that will generate configmap.
