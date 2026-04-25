package nginx

// MediaProxySiteConf is the nginx conf.d path for the palace-manager media proxy vhost.
// conf.d is included by default on Debian and RHEL/CentOS stock nginx.conf (unlike sites-enabled on RHEL).
// It is fixed regardless of nginx.mediaHost so the filename stays predictable on every host.
const MediaProxySiteConf = "/etc/nginx/conf.d/100-palace-manager-media.conf"
