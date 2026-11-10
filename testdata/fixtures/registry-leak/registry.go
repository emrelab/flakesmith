package registryleak

var clients = map[string]string{}

func register(client, model string) { clients[client] = model }
