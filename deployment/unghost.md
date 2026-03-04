
# List all ghosted events (read-only)
  go run ./scripts/unghost

  # List ghosted events for one specific service
  go run ./scripts/unghost --service=intasend

  # Preview what a single event looks like before unghostint
  go run ./scripts/unghost --id=42

  # Unghost a single event
  go run ./scripts/unghost --id=42 --apply

  # Unghost all events from a service
  go run ./scripts/unghost --service=intasend --apply

  # Unghost everything
  go run ./scripts/unghost --all --apply

  # Different DB path
  go run ./scripts/unghost --db=/path/to/ek1.db --service=intasend --apply

  What it does when unghostting:
  - Sets decision from Declined (2) → Accepted (1)
  - Sets analysis.triage_gate from "manipulation" → "unghosted_manual" so you can distinguish it from organically accepted
   events
  - Never touches anything that isn't actually a ghosted event


 List all ghosted events
  docker compose exec ek1 unghost                                                                                         
  
  # List ghosted events for one service                                                                                   
  docker compose exec ek1 unghost --service=intasend        

  # Preview a specific event
  docker compose exec ek1 unghost --id=42

  # Unghost a specific event
  docker compose exec ek1 unghost --id=42 --apply

  # Unghost all events from a service
  docker compose exec ek1 unghost --service=intasend --apply

  # Unghost everything
  docker compose exec ek1 unghost --all --apply
