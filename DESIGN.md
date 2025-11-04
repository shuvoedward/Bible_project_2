# Design Decisions

## Why This Stack?
- **PostgreSQL**: Needed ACID compliance for user data,
 complex queries for Bible searches, and relational data 
 (notes -> verses -> books)
- **Redis**: Caching Api token for faster response.
- **Token-Based auth**: Simpler than JWT for this use case,
  easier to revoke, store in DB. 


## API Design Decisions
- **REST over GraphQL**: Simpler for portfolio, well-understood,
  easier for mobile clients
- **Versioning (/v1/)**: Planning for future changes without 
  breaking existing clients


## Note Types
Why three types (general, scripture-based, cross-reference)?
- General: Quick thoughts not tied to specific verses
- Scripture-based: Deep dive on specific passages
- Cross-reference: Connect related verses (study flow)


## Rate Limiting Strategy
- Auth endpoints: 10/min (prevent brute force)
- Notes: 100/min (balance protection vs UX)
- IP-based fallback for unauthenticated users


## Trade-offs Made
- **S3 for images**: More expensive than local storage, but:
   Scalable,  CDN-ready, No server disk issues
  Costs money,  External dependency
- **Email activation**: Better security vs friction