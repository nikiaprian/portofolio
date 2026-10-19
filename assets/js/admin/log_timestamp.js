/**
 * Shared admin log timestamp: "12 Apr 2026, 13.10" (local browser time).
 */
(function (g) {
  "use strict";
  var months = [
    "Jan",
    "Feb",
    "Mar",
    "Apr",
    "May",
    "Jun",
    "Jul",
    "Aug",
    "Sep",
    "Oct",
    "Nov",
    "Dec",
  ];

  function pad2(n) {
    return String(n).length < 2 ? "0" + n : String(n);
  }

  g.AdminLogFormat = {
    formatTimestamp: function (iso) {
      if (!iso) return "—";
      try {
        var d = new Date(iso);
        if (isNaN(d.getTime())) return String(iso);
        var day = d.getDate();
        var mon = months[d.getMonth()];
        var y = d.getFullYear();
        return (
          day +
          " " +
          mon +
          " " +
          y +
          ", " +
          pad2(d.getHours()) +
          "." +
          pad2(d.getMinutes())
        );
      } catch (e) {
        return String(iso);
      }
    },
  };
})(typeof window !== "undefined" ? window : this);
