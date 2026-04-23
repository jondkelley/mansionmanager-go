package nginx

// MediaProxySiteConf is the nginx sites-enabled path for the palace-manager media proxy vhost.
// It is fixed regardless of nginx.mediaHost so the filename stays predictable on every host.
const MediaProxySiteConf = "/etc/nginx/sites-enabled/100-palace-manager-media.conf"
