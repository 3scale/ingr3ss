package generator

import (
	"crypto/md5"
	"encoding/json"
	"fmt"

	"github.com/3scale/kourier/pkg/config"
	"github.com/3scale/kourier/pkg/envoy"
	"k8s.io/api/networking/v1beta1"

	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// Generates an internal virtual host that signals that the Envoy instance has
// been configured for all the ingresses received in the params.
// The virtual host generated contains a route for each ingress received in the
// params. The path of the routes are hashed ingresses. With this, if the
// request for a hashed ingress is successful, we know that the gateway has been
// configured for that ingress.
func statusVHost(ingresses []*v1beta1.Ingress) route.VirtualHost {
	return envoy.NewVirtualHost(
		config.InternalKourierDomain,
		[]string{config.InternalKourierDomain},
		statusRoutes(ingresses),
	)
}

func statusRoutes(ingresses []*v1beta1.Ingress) []*route.Route {
	var hashes []string
	var routes []*route.Route
	for _, ingress := range ingresses {
		hash, err := computeHash(ingress)
		if err != nil {
			log.Errorf("Failed to hash ingress %s: %s", ingress.Name, err)
			break
		}
		hashes = append(hashes, fmt.Sprintf("%x", hash))
	}

	for _, hash := range hashes {
		name := fmt.Sprintf("%s_%s", config.InternalKourierDomain, hash)
		path := fmt.Sprintf("%s/%s", config.InternalKourierPath, hash)
		routes = append(routes, envoy.NewRouteStatusOK(name, path))
	}

	// HACK: There's a bug/behaviour in envoy <1.12.0 that doesn't retry loading the config if it's the same.
	random, _ := uuid.NewUUID()
	routes = append(routes, envoy.NewRouteStatusOK(random.String(), "/ready"))

	staticRoute := envoy.NewRouteStatusOK(
		config.InternalKourierDomain,
		config.InternalKourierPath,
	)
	routes = append(routes, staticRoute)

	return routes
}

func computeHash(ing *v1beta1.Ingress) ([16]byte, error) {
	bytes, err := json.Marshal(ing.Spec)
	if err != nil {
		return [16]byte{}, fmt.Errorf("failed to serialize Ingress: %w", err)
	}
	bytes = append(bytes, []byte(ing.GetNamespace())...)
	bytes = append(bytes, []byte(ing.GetName())...)
	return md5.Sum(bytes), nil
}