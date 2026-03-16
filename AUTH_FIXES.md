# Auth Fixes

Current auth is messed up and users keep getting issues with login and signup. The problem starts at the signup. The token generation and login might not be an issue. Signup seems not to be validating properly and act in atomic way. There are also issues with data types i.e mixing integers and strings between User.ID and Organization.ID. I would like us to resolve it this way and make sure it works.

## Suggestion

### Working directory pkg/auth

Tip: As you implement this suggestions: Simplify the auth and make it easy to maintain. Right now it is so complex for a simple task like auth. Make it straight forward. Remove the repository things, just have a normal go code

1. Check models in auth and ensure data types are well factored in
2. Ensure relationships between User, Organization, TeamMember etc are well linked and will work well with Postgres database. Should respect each ID data type
3. When signup, use database transaction atomic in the flow. Example: User should not be created when creating an Organization entry fails. If error occurs, the transaction should automatically rollback. 
4. When signup the flow should be (in database atomic block): 
    a. Check if user exists with email, if yes, ask them to login, otherwise proceed in the next step i.e creating the user
    b. Create the user organization and mark the new user as the owner
    c. Update the team account
    d. Todo later: Send email to the user (create a function that we will use in future for this) - This should only be done when database tranction commit (transaction.on_commit signal of step a,b,c)
5. Write test of this flow i.e unit test
6. Ensure the API handler/endpoint is not broken by writing an unit test (only and no any other kind of script)
7. Move unreleated auth files like providers, integrations, security_handlers to own packages and update the routes. This package auth should only contain signup and login related items e.g sessions. Organization can also move to org package and have organization and team management there but ensure they work together and do the db transaction atomic.
8. Test this with postgres db in mind since that is what we will be pushing to production
9. Clean up all unused codes
10. Feel free to ask questions if things are not clear


