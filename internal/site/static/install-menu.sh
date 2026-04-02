#!/bin/sh
# Stockyard Interactive Tool Installer
# Usage: curl -fsSL stockyard.dev/install-menu.sh | sh
set -e

GREEN='\033[0;32m'; GOLD='\033[0;33m'; RED='\033[0;31m'; CYAN='\033[0;36m'; DIM='\033[0;90m'; BOLD='\033[1m'; NC='\033[0m'

clear 2>/dev/null || true

printf "\n${BOLD}${CYAN}  ╔══════════════════════════════════════╗${NC}\n"
printf "${BOLD}${CYAN}  ║     Stockyard Tool Installer         ║${NC}\n"
printf "${BOLD}${CYAN}  ╚══════════════════════════════════════╝${NC}\n\n"

printf "  ${DIM}Pick a category, then choose your tools.${NC}\n"
printf "  ${DIM}Each tool is a single binary (~13MB).${NC}\n\n"

printf "  ${BOLD}Categories:${NC}\n\n"
printf "    ${CYAN}1${NC}  Developer Tools     ${DIM}(bug tracker, API testing, secrets, flags, CI/CD)${NC}\n"
printf "    ${CYAN}2${NC}  Operations          ${DIM}(monitoring, incidents, wiki, compliance)${NC}\n"
printf "    ${CYAN}3${NC}  Finance & Business  ${DIM}(invoicing, CRM, expenses, bookkeeping)${NC}\n"
printf "    ${CYAN}4${NC}  Creator & Marketing ${DIM}(blog, email, analytics, link shortener)${NC}\n"
printf "    ${CYAN}5${NC}  Personal            ${DIM}(notes, passwords, journal, recipes)${NC}\n"
printf "    ${CYAN}6${NC}  Starter Bundle      ${DIM}(5 most popular tools)${NC}\n"
printf "    ${CYAN}7${NC}  All Free Tools      ${DIM}(13 tools, Apache 2.0)${NC}\n"
printf "    ${CYAN}0${NC}  Exit\n"

printf "\n  ${BOLD}Choose [0-7]:${NC} "
read CHOICE

case "$CHOICE" in
1)
  CAT="Developer Tools"
  TOOLS="bounty:Bounty:Bug tracker & issue manager
roundup:Roundup:Task manager with priorities
assay:Assay:API testing suite
strongbox:Strongbox:Secret manager (AES-256)
saltlick:Salt Lick:Feature flag service
bellwether:Bellwether:Uptime monitor
pipeline:Pipeline:CI/CD orchestrator
embargo:Embargo:Feature flag manager
codex:Codex:Code snippet library
barrage:Barrage:Load tester" ;;
2)
  CAT="Operations"
  TOOLS="handbook:Handbook:Internal wiki
inquest:Inquest:Incident manager
sentinel:Sentinel:Alert manager
paddock:Paddock:Status page
brand:Brand:Audit trail
announcements:Announcements:Team broadcasts
campfire:Campfire:Async standups
outpost:Outpost:Infrastructure monitor
quarry:Quarry:Log aggregation
switchboard:Switchboard:Service discovery" ;;
3)
  CAT="Finance & Business"
  TOOLS="billfold:Billfold:Invoice generator
dossier:Dossier:Contact CRM
prospector:Prospector:Sales pipeline
steward:Steward:Expense tracker
books:Books:Bookkeeping
ledger:Ledger:General ledger
exchequer:Exchequer:Budget planner
consortium:Consortium:Vendor tracker" ;;
4)
  CAT="Creator & Marketing"
  TOOLS="post:Post:Blog engine
dispatch:Dispatch:Email newsletters
headcount:Headcount:Web analytics
lasso:Lasso:Link shortener
crossroads:Crossroads:URL shortener
brander:Brander:Brand asset manager
podium:Podium:Feedback board
surveyor:Surveyor:Survey builder
cartograph:Cartograph:Sitemap generator" ;;
5)
  CAT="Personal"
  TOOLS="notebook:Notebook:Notes & docs
cipher:Cipher:Password manager
almanac:Almanac:Personal journal
curator:Curator:Recipe planner
trailhead:Trailhead:Habit tracker
apothecary:Apothecary:Medication tracker
archive:Archive:Bookmarks & clippings" ;;
6)
  printf "\n  ${GREEN}Installing Starter Bundle (5 tools)...${NC}\n\n"
  exec sh -c "curl -fsSL stockyard.dev/install-tools.sh | sh -s -- --starter"
  exit 0 ;;
7)
  printf "\n  ${GREEN}Installing All Free Tools (13 tools)...${NC}\n\n"
  exec sh -c "curl -fsSL stockyard.dev/install-tools.sh | sh -s -- --all-free"
  exit 0 ;;
0|"")
  printf "\n  ${DIM}See you at stockyard.dev${NC}\n\n"
  exit 0 ;;
*)
  printf "\n  ${RED}Invalid choice${NC}\n"
  exit 1 ;;
esac

printf "\n  ${BOLD}${CAT}:${NC}\n\n"

I=1
SLUGS=""
IFS='
'
for LINE in $TOOLS; do
  SLUG=$(echo "$LINE" | cut -d: -f1)
  NAME=$(echo "$LINE" | cut -d: -f2)
  DESC=$(echo "$LINE" | cut -d: -f3)
  printf "    ${CYAN}%2d${NC}  %-18s ${DIM}%s${NC}\n" "$I" "$NAME" "$DESC"
  eval "TOOL_${I}=$SLUG"
  I=$((I + 1))
done
unset IFS

printf "    ${CYAN} 0${NC}  Back\n"
printf "\n  ${BOLD}Enter tool numbers (space-separated), or 'all':${NC} "
read PICKS

if [ "$PICKS" = "0" ] || [ -z "$PICKS" ]; then
  exec sh -c "curl -fsSL stockyard.dev/install-menu.sh | sh"
  exit 0
fi

SELECTED=""
if [ "$PICKS" = "all" ]; then
  J=1
  while [ $J -lt $I ]; do
    eval "S=\$TOOL_${J}"
    SELECTED="$SELECTED $S"
    J=$((J + 1))
  done
else
  for PICK in $PICKS; do
    eval "S=\$TOOL_${PICK}" 2>/dev/null
    if [ -n "$S" ]; then
      SELECTED="$SELECTED $S"
    fi
  done
fi

SELECTED=$(echo "$SELECTED" | xargs)
if [ -z "$SELECTED" ]; then
  printf "\n  ${RED}No valid tools selected${NC}\n"
  exit 1
fi

COUNT=$(echo "$SELECTED" | wc -w | tr -d ' ')
printf "\n  ${GREEN}Installing $COUNT tools...${NC}\n\n"

exec sh -c "curl -fsSL stockyard.dev/install-tools.sh | sh -s -- $SELECTED"
