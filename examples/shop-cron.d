# Fictional /etc/cron.d file for the same shop server, used to try
# cron-explain -system. It has a user column. Commands are placeholders.
SHELL=/bin/sh
MAILTO=""
CRON_TZ="America/Toronto"
0 3 * * *        backup   /opt/shop/bin/db-dump --all
*/10 8-20 * * *  shop     /opt/shop/bin/price-check
0 30 2 * * *     root     /opt/shop/bin/old-style-seconds
0 5 * * 1-5      /opt/shop/bin/forgot-user
CRON_TZ=Europe/Londn
0 7 * * *        shop     /opt/shop/bin/uk-feed
