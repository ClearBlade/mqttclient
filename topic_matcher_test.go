package mqttclient

import (
	mqtt "github.com/clearblade/mqtt_parsing"
	"log"
	"testing"
	"time"
)

var ()

//have to decompose the test into subtests
func Test_TopicMatcherRegular(t *testing.T) {
	topics := []string{"foo", "foo/bar", "foo/bar/baz/Пчёлка"}
	ot := newOutgoingTopics()
	m := populateTopics(t, ot, topics)
	for topic, cha := range m {
		msg := MakeMeAPublish(topic, "hello on "+topic, 0)
		err := ot.relay_message(msg, topic)
		if err != nil {
			t.Fatal(err.Error())
		}
		select {
		case _ = <-cha:
		case <-time.After(time.Second):
			t.Fatal("didn't push message along")
		}
	}
	//now for some fake topics
	faketop := []string{"foobar", "spam", "eggies"}
	for _, topic := range faketop {
		msg := MakeMeAPublish(topic, "hello on "+topic, 0)
		err := ot.relay_message(msg, topic)
		if err == nil {
			t.Error("should have errored out as no subscription exists")
		}
	}
}

func Test_TopicMatchPlus(t *testing.T) {
	topics := []string{"foo/+", "bar/baz/+", "quux/+/spam", "+/spam", "foo/bar/+/+"}
	ot := newOutgoingTopics()
	m := populateTopics(t, ot, topics)
	//the reason we don't break this fn out of scope is to capture the reference to m
	test_msg := func(topic, recieving string, ot *outgoing_topics) {
		msg := MakeMeAPublish(topic, "hello from "+topic, 0)
		ot.relay_message(msg, topic)
		select {
		case _ = <-m[recieving]:
		case <-time.After(time.Second):
			t.Error(recieving + " should have gotten message")
		}
	}

	test_msg("foo/bar", topics[0], ot)
	test_msg("bar/baz/spam", topics[1], ot)
	test_msg("quux/baz/spam", topics[2], ot)
	test_msg("baz/spam", topics[3], ot)
	test_msg("foo/bar/baz/spam", topics[4], ot)
	//now we'll check for double matches
	tt5 := "foo/spam"
	msg5 := MakeMeAPublish(tt5, "hello from "+tt5, 0)
	ot.relay_message(msg5, tt5)
	got1 := 0
	select {
	case _ = <-m[topics[0]]:
		got1++
	case <-time.After(time.Second):
	}
	select {
	case _ = <-m[topics[3]]:
		got1++
	case <-time.After(time.Second):
	}

	switch got1 {
	case 1:
		log.Println("got 1")
	case 2:
		log.Println("got 2")
	default:
		t.Error("did not recieve either")
	}

	invalid_topics := []string{"bar/baz/quux/eggs", "spam", "quickly/get some/spam"}
	test_invalid := func(topic string, ot *outgoing_topics) {
		msg := MakeMeAPublish(topic, "hello from "+topic, 0)
		ot.relay_message(msg, topic)
		for top, ch := range m {
			select {
			case _ = <-ch:
				t.Error("errantly recieved on " + top)
			case <-time.After(time.Millisecond * 50):
			}
		}
	}
	for _, v := range invalid_topics {
		test_invalid(v, ot)
	}
	//remove topics
	ot.remove_subscription(topics[0])
	test_msg(tt5, topics[3], ot)

}

func Test_TopicMatchCrunch(t *testing.T) {
	topics := []string{"/foo/bar/#", "/foo/#", "/baz/#"}
	ot := newOutgoingTopics()
	m := populateTopics(t, ot, topics)
	test_msg := func(topic, recieving string, ot *outgoing_topics) {
		msg := MakeMeAPublish(topic, "hello from "+topic, 0)
		ot.relay_message(msg, topic)
		select {
		case _ = <-m[recieving]:
		case <-time.After(time.Second):
			t.Error(recieving + " should have gotten message")
		}
	}
	//the subscription "/foo/#" should subsume all of /foo/#" messages
	test_msg("foo/bar/baz", "/foo/bar/#", ot)
	test_msg("foo/bar/", "/foo/#", ot)
	//so let us remove it
	ot.remove_subscription("/foo/bar/#")
	test_msg("foo/bar/baz", "/foo/#", ot)
	test_msg("baz/bux", "/baz/#", ot)
}

func Test_WildcardRumble(t *testing.T) {
	topics := []string{"/+/bar/#", "/baz/+/bux/#"}
	ot := newOutgoingTopics()
	m := populateTopics(t, ot, topics)
	test_msg := func(topic, recieving string, ot *outgoing_topics) {
		msg := MakeMeAPublish(topic, "hello from "+topic, 0)
		ot.relay_message(msg, topic)
		select {
		case _ = <-m[recieving]:
		case <-time.After(time.Second):
			t.Fatal(recieving + " should have gotten message")
		}
	}
	//should get
	test_msg("/foo/bar/baz/buux", "/+/bar/#", ot)
	test_msg("/baz/spammy/bux/Звёздочка/Армстронг, Нил", "/baz/+/bux/#", ot)
}

func populateTopics(t *testing.T, ot *outgoing_topics, top []string) map[string]chan *mqtt.Publish {
	m := make(map[string]chan *mqtt.Publish)
	for _, v := range top {
		ch, err := ot.new_outgoing(v)
		if err != nil {
			t.Fatal(err.Error())
		}
		m[v] = ch
	}
	return m
}
