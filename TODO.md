# Fixes

## Deployment

We need just one docker-compose file i.e docker-compose.yml, it should all that is need to run the app. Remove the prod and others. Put everything together and test i.e including ports checks and network.

## Nginx

Should proxy the containers properly and expose only one port i.e 80

## UI/UX Design

Make the UI modern using Tailwind. Provide shortcuts to the relevant items e.g monitoring and alerts modules i.e container

## Auth and Security

Provide auth mechanism. Users should be able to login with email and password. On signup, they should be able to access only the data owned by them. Provide isolation by puting the user in an org and when quering, filter items by org they belong to.

## Provide cli commands for the admin

1. To add an admin user in any org
2. To reset any user password
3. To delete all the org data