package dropintoascenario

import "givens/acceptance/world"

// byHandSteps cover first_you lines, and by_hand and ask cards from a setup script.
func byHandSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^the "Card declined" setup ends with first_you '([^']*)'$`, s.endsWithFirstYou),
		world.Def(`^the checklist starts with "(.*)"$`, s.checklistStartsWith),
		world.Def(`^the When steps come after it$`, s.whensAfter),
		world.Def(`^the Then steps are still the only ones marked "Pass" or "Fail"$`,
			s.onlyThensPassFail),
		world.Def(`^the "([^"]*)" setup has a by_hand step for "([^"]*)" with the snippet `+
			`"([^"]*)"$`, s.byHandSnippet),
		world.Def(`^the output pauses at a card showing "([^"]*)" and the snippet$`,
			s.pausedAtCard),
		world.Def(`^they can choose "Copy" to copy the snippet$`, s.copySnippet),
		world.Def(`^choosing "Done" carries on with the rest of setup$`, s.doneCarriesOn),
		world.Def(`^a developer has dropped in to "([^"]*)" and setup is paused at its by_hand `+
			`step$`, s.pausedAtByHand),
		world.Def(`^setup stops and they see "([^"]*)"$`, s.setupStopsWith),
		world.Def(`^the "([^"]*)" setup has by_hand --url "([^"]*)" for "([^"]*)"$`,
			s.byHandLink),
		world.Def(`^a developer drops in and setup pauses at that step$`, s.dropInToPause),
		world.Def(`^the card offers "([^"]*)"$`, s.cardOffers),
		world.Def(`^choosing it opens the page in their default browser$`, s.opensInBrowser),
		world.Def(`^the "([^"]*)" setup has ask ([A-Z_]+) "([^"]*)"$`, s.setupAsks),
		world.Def(`^a developer drops in, types "([^"]*)" into the card and chooses "([^"]*)"$`,
			s.typeAndChoose),
		world.Def(`^the rest of setup runs with ([A-Z_]+) set to "([^"]*)"$`, s.restRunsWith),
		world.Def(`^setup is paused at an ask card$`, s.pausedAtAsk),
		world.Def(`^a developer chooses "Continue" with the field empty$`, s.continueEmpty),
		world.Def(`^setup stays paused$`, s.staysPaused),
		world.Def(`^the "Pay with saved card" setup has a by_hand step$`, s.fromScenarioByHand),
		world.Def(`^the card for Pay with saved card's step is shown$`, s.fromCardShown),
		world.Def(`^its other "your turn" lines stay hidden$`, s.yourTurnHidden),
		world.Def(`^(\d+) minutes pass before they choose "Done"$`, s.minutesPassThenDone),
	}
}

// rowSteps cover outlines: one Drop in page for every Examples row.
func rowSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^checkout\.feature has the outline "Card rejected by the bank"`+
			`(?: and the scenario "([^"]*)")?$`, s.hasOutline),
		world.Def(`^its Drop in page lists the "([^"]*)" and "([^"]*)" rows one after another$`,
			s.rowsListed),
		world.Def(`^each row shows its steps with the placeholders filled in, like "(.*)"$`,
			s.rowsFilledIn),
		world.Def(`^each row shows its status and its own "Drop in" button$`, s.rowsHaveStatus),
		world.Def(`^a developer has "([^"]*)" open$`, s.outlineOpen),
		world.Def(`^they choose "([^"]*)" on the "([^"]*)" row$`, s.chooseOnRow),
		world.Def(`^(?:the setup script runs|that setup runs) with GIVENS_EX_card set to `+
			`"([^"]*)"$`, s.exampleCardIs),
		world.Def(`^that row's checklist opens beneath it$`, s.rowChecklistOpen),
		world.Def(`^the "([^"]*)" row stays readable but greyed out, with its "Drop in" `+
			`disabled$`, s.rowGreyedOut),
		world.Def(`^a developer has dropped in to the "([^"]*)" row and marked one Then step$`,
			s.rowMarkedOne),
		world.Def(`^they're asked "([^"]*)"$`, s.asked),
		world.Def(`^choosing "Leave" closes the ([a-z]+) checklist and runs setup for `+
			`"([^"]*)"$`, s.leaveSwitches),
		world.Def(`^a developer has dropped in to "([^"]*)" with the "([^"]*)" row and marked `+
			`every Then step "Pass"$`, s.rowAllPassed),
		world.Def(`^a line for (\S+) is added to today's file in runs/, with (\w+)=(\w+)$`,
			s.runLineWithValue),
		world.Def(`^the scenario shows "([^"]*)"$`, s.scenarioShows),
		world.Def(`^the Unverified view lists the "([^"]*)" row as "([^"]*)"$`,
			s.unverifiedLists),
		world.Def(`^the "Pay with saved card" Given step has a data table of two cart items$`,
			s.dataTable),
		world.Def(`^GIVENS_ARG_1 holds the path to a CSV file with a header row and both items$`,
			s.argIsCSV),
		world.Def(`^the setup script can seed the cart from it$`, s.cartSeeded),
		world.Def(`^"([^"]*)" has two Examples blocks with no names$`, s.unnamedExamples),
		world.Def(`^the scenario is listed with the warning "([^"]*)"$`, s.listedWithWarning),
		world.Def(`^"Record run" is disabled on it until they're named$`,
			s.recordDisabledUntilNamed),
	}
}

// markdownSteps cover setup done by hand in scripts/<slug>.md, and copying its code.
func markdownSteps(s *state) []world.Step {
	return []world.Step{
		world.Def(`^"([^"]*)" has scripts/([a-z0-9-]+)\.md with 3 numbered steps, and no \.sh `+
			`script$`, s.markdownOnly),
		world.Def(`^they aren't asked to trust the project$`, s.notAskedToTrust),
		world.Def(`^they see "([^"]*)" above (\d+) cards, with the link and the wallet `+
			`screenshot shown$`, s.introAboveCards),
		world.Def(`^choosing "Done" on the last card shows the checklist$`, s.doneOnLastCard),
		world.Def(`^"([^"]*)" has both scripts/([a-z0-9-]+)\.sh and scripts/([a-z0-9-]+)\.md$`,
			s.scriptAndMarkdown),
		world.Def(`^the script's output appears first$`, s.scriptOutputFirst),
		world.Def(`^when it finishes, the cards from ([a-z0-9-]+)\.md are shown$`,
			s.markdownCardsShown),
		world.Def(`^a developer has dropped in to "([^"]*)" and marked the first card "Done"$`,
			s.firstCardDone),
		world.Def(`^they choose "([^"]*)" on the (first|second|third|last) card$`,
			s.chooseOnCard),
		world.Def(`^step 2 of pay-in-app\.md holds a bash code block, indented under its list `+
			`item$`, s.codeBlocksSetup),
		world.Def(`^a developer chooses "Copy" on that code block in Drop in$`, s.copyBashBlock),
		world.Def(`^it holds no fences, list number or leading spaces$`, s.cleanCopy),
		world.Def(`^step 2 holds a bash code block and step 3 holds a js code block$`,
			s.codeBlocksSetup),
		world.Def(`^a developer looks at those cards in Drop in$`, s.lookAtCards),
		world.Def(`^each code block has its own Copy button, labelled "([^"]*)" and "([^"]*)"$`,
			s.labelledCopyButtons),
		world.Def(`^copying one never includes the other$`, s.copiesApart),
		world.Def(`^a card says "([^"]*)"$`, s.cardSays),
		world.Def(`^a developer clicks (\S+)$`, s.clickInlineCode),
		world.Def(`^step 2 of pay-in-app\.md opens a code fence that never closes$`,
			s.unclosedFence),
		world.Def(`^step (\d+)'s card shows "([^"]*)"$`, s.stepCardShows),
		world.Def(`^it has no Copy button until the fence is closed$`, s.noCopyUntilClosed),
	}
}
