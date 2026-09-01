

### How does the current user-to-pool distribution system work?

Each storage user is assigned to a single pool, and new users are distributed based on available free space.

Pools are currently formed **regardless of node rating** — this is a deliberate choice made to guarantee that user data remains available across the network. Because we cannot control how much data each user uploads, the amount of data in pools and on individual nodes can differ.

The team is working on an algorithm under which the most stable nodes will be able to receive a larger reward. Details will be announced once the algorithm is finalized.

### Is manual pool selection available?

Pool assignment is fully automatic — there is no manual pool selection at this time.

### What can cause previously stored data to become unavailable?

The most common causes are:

- The node was reassigned to a different pool after a restart — data associated with the previous pool is no longer tracked under the node's new assignment.
- The end user who owned the data deleted it themselves — the Datakeeper stops receiving proof challenges and rewards for that data because it no longer exists on the user's side.

The amount of data on a node is a dynamic metric that can fluctuate for many reasons on the user's side. It is not always synchronized with payment calculations at the moment, so you should not perceive a short-term decrease in volume as a direct indicator of income.

## Rewards & Tokens

### How is network revenue generated and distributed?

Coming soon: live earnings calculation and forecasting will be built directly into official [Datakeeper Console](https://datakeeper-console.denet.pro/dashboard), so every Datakeeper can track their real-time share and projected growth in one place.

### What determines the difference in earnings between Datakeepers?

Earnings come from real user payments for storage. When a node successfully submits Proof-of-Storage, it receives TBY directly from the users whose data it stores, in the pool in which it was defined.

Today, the main factor is how much user data a node stores and successfully proves. Pools are formed regardless of rating so that data stays available, and rewards cannot be redistributed evenly between nodes. The team is working on an algorithm under which the most stable nodes will be able to earn more.

### Why can't rewards be distributed evenly between Datakeepers?

Even distribution is not possible under the current model:

- We cannot directly control how much data each storage user uploads, so the amount of data — and therefore potential income — naturally varies between pools and nodes.
- Rewards are paid directly by users to the nodes that store and prove their data. There is no centralized reward pool that we could split equally.

What we can influence is node stability. The team is working on an algorithm under which the most stable nodes will be able to receive a larger reward.
