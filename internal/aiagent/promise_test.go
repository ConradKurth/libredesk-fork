package aiagent

import "testing"

func TestClaimsTeammateAction(t *testing.T) {
	claims := []string{
		// Prod 2026-09-28: promised a replacement without proposing it or handing off, then resolved.
		"Thanks for contacting Cyclone Support. I see your order #184925 was shipped on 2026-09-20 and should contain four Gust Pro Sour Green Apple units. Since you received only one, we’ll arrange a replacement for the three missing units.\n\nA teammate will process the replacement and you’ll receive a confirmation email shortly.",
		"Thanks for the photo; a teammate will confirm and send 3 x Gust Pro Sour Green Apple at no charge.",
		"I've noted your request and our support team will follow up by email.",
		"Someone from our team will reach out about the refund.",
		"Our team is going to review the photos and get back to you.",
		"We will send the missing items today.",
		// Prod 2026-10-04: a double-charge ticket stayed with the assistant after this reply.
		"Thanks for the clarification, Rachel. It looks like a separate order may have been placed in addition to your regular subscription charge. I’ll have a teammate review your account and the recent orders to determine why the extra charge appeared.",
		"I'll pass this along to our support team so they can sort out the refund.",
		"I’m escalating this to a specialist now.",
		"I don’t see a 0 % menthol vape juice in our catalog at the moment, so a teammate will check on any upcoming options for you.",
	}
	for _, reply := range claims {
		if !claimsTeammateAction(reply) {
			t.Errorf("claimsTeammateAction(%q) = false, want true", reply)
		}
	}

	notClaims := []string{
		"Would you like a teammate to follow up on this?",
		// Prod 2026-09-28, the same ticket's first reply: asking for the photo is not a promise yet.
		"To resolve this, could you please provide the order number and a photo of the packing slip? Once we have that information, a teammate will arrange a replacement for the missing items.",
		"Please reply with a photo of what arrived with the packing slip. A teammate will then arrange a replacement if needed.",
		"If the pod still does not produce vapor after these steps, we’ll arrange a replacement.",
		"If you don’t see the option, let us know and a teammate will look into it.",
		"You can change your flavor any time at cyclonepods.com/account/subscriptions.",
		"Your order shipped yesterday and you'll receive tracking by email.",
		"Orders are usually processed within 24 hours and ship from our warehouse.",
		"Thanks for contacting Cyclone Support. Did that resolve your question?",
		"Once it arrives, you can start a return from your account page.",
		"I'll have a look at the options for you: the Gust Pro comes in 20 flavors.",
		"If you'd like, I'll have a teammate check on it.",
		"I will get you the tracking link from the order tool.",
	}
	for _, reply := range notClaims {
		if claimsTeammateAction(reply) {
			t.Errorf("claimsTeammateAction(%q) = true, want false", reply)
		}
	}
}
