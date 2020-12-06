
def init(self,dummy="test"):
  self.mariadb = chart("../mariadb")
  self.mariadb.slave['replicas'] = 2

  self.mariadb.create_database(db="uaa",username="uaa",password="87612349234")

  self.uaa = chart("../uaa",namespace="uaa")
  self.uaa.database_credentials.db = "uaa"
  self.uaa.database_credentials.username = "uaa"
  self.uaa.database_credentials.password = "87612349234"
  self.name = "my-first-chart"
  self.password = "test-pass"
  return self


def __secret_name(self):
  return "mysecret"

def apply(self, k8s):
  self.mariadb.apply(k8s)
  k8s.rollout_status("statefulset","mariadb-master")
  self.uaa.apply(k8s)
  self.__apply(k8s)