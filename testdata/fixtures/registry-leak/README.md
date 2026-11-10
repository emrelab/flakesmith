Original, dependency-free reduction of the global registry cleanup pattern in
router-for-me/CLIProxyAPI#7356. This is a model of the mechanism, not copied
upstream code and not evidence of a real-world finding. TestWebsocketClient
leaves a client registered, so a later TestCatalog sees an unexpected model.
The real repository lifecycle is recorded in docs/real-world/README.md.
