RoseWire Federation & Privacy Plan

This document outlines the technical plan to evolve RoseWire from a single-server application into a decentralized, federated network. The primary goals are to eliminate single points of failure, allow for community-hosted instances, and provide robust IP address protection for all users during cross-instance interactions.

The core principle is to use each user's home instance as a trusted proxy, ensuring a user's personal IP address is never directly exposed to another instance or its users.
## Step 1: Global Identity & Discovery

A user's identity must be unique and discoverable across the entire federation, not just on one server.

    Global User Address:

        Usernames will be standardized to a global format: @nickname@instance.domain.

        For example: @rose@rosewire.rosevines.network.

        The client application must be updated to handle this format for login and display.

    Decentralized Discovery via WebFinger:

        Each RoseWire server instance must implement a WebFinger endpoint. This is a standard, well-documented protocol for discovering information about users on a server.

        Endpoint: https://instance.domain/.well-known/webfinger?resource=acct:@nickname@instance.domain

        Function: When a server needs to find a user from another instance, it will query this public HTTPS endpoint.

        Response: The endpoint will return a JSON document containing the user's profile information and, most importantly, their SSH public key. This allows any server to verify a user's identity and retrieve the key needed for authentication.

## Step 2: Server-to-Server (S2S) Communication

Instances need a secure channel to exchange activities, user data, and search queries. This is separate from the client's SSH connection.

    Protocol: All S2S communication will occur over a secure HTTPS REST API.

    Authentication: To prevent spoofing, all requests to the S2S API must be authenticated. Servers will sign their outgoing requests with a dedicated instance-level private key. The receiving server will verify this signature against the sending server's public key.

    Core S2S API Endpoints: Each server must implement the following endpoints:

        POST /api/s2s/inbox: The primary endpoint for receiving "Activities" (like new chat messages or file shares) from other instances.

        GET /api/s2s/user/{user_address}: An internal-facing endpoint for one server to look up a user's profile from another.

        GET /api/s2s/search?query={query}: An endpoint to forward a search request to a peer instance.

## Step 3: Data Federation via Activity Push

We will use a push-based model inspired by the ActivityPub protocol. When an event happens on one server, it is "pushed" to all relevant peers.

    File Sharing:

        When a user on instance-a shares a file, the server creates a JSON object representing a Share activity.

        instance-a then sends this Share activity to the /inbox of all its peered servers.

        The receiving servers update their file registries, caching the file information and noting it is owned by @user@instance-a.

    Federated Search:

        A user on instance-b initiates a search.

        instance-b searches its local cache and forwards the query to the /api/s2s/search endpoint of all its peers.

        It aggregates all results and returns them to the client, who sees a single, unified list.

    Chat Federation:

        Works identically. A chat message is wrapped in a Create activity and pushed to the inboxes of all peered servers, which then deliver it to their local clients.

## Step 4: Privacy-Preserving File Transfers

This is the most critical component for user protection. Direct client-to-client connections for cross-instance transfers are strictly forbidden.

    The Trusted Proxy Model: The user's home instance must act as a proxy for all file transfers originating from or destined for another instance.

    Anonymized Download Flow:

        User A on instance-a wants a file from User B on instance-b.

        User A's client sends a download request to its own server, instance-a.

        instance-a then initiates the download from User B. The connection path is instance-a -> instance-b -> User B's client.

        The file data is then relayed back through the servers to User A.

        The complete data path is: User B -> instance-b -> instance-a -> User A.

    IP Protection: In this flow, instance-b and User B only ever see the IP address of instance-a. User A's personal IP address is never exposed.

    Bandwidth Trade-Off: This model intentionally places the bandwidth cost on the instance operators. This is a necessary trade-off to guarantee user privacy and security.

## Step 5: Instance Peering

Servers need a way to discover each other to form the mesh network.

    Initial Peering List: The federation will start with a public, community-managed list of trusted instances (e.g., a peers.json file in a Git repository). New instance admins can use this list to find their first peers.

    Gossip Protocol (Future): Over time, instances can implement a gossip protocol to automatically discover and share information about other instances on the network, allowing for organic growth.