//go:build !unix

package history

// noFollowFlag is zero where O_NOFOLLOW does not exist; the Lstat pre-check in
// OpenDailyDB is the only guard there.
const noFollowFlag = 0
