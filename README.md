1. 
  需求简介：
  
  按照k8s中deployment controller的设计模式，写一个controller，
  可以根据deployment资源的annotation中：
  "service: bool"字段的值自动生成对应的service，并且service的名称为deployment资源名加service；
  "service-type: string"字段的值控制service的类型；

2. 
  几种情况如下：

  2.1 create：
    根据字段值创建所需类型的service

  2.2 update:
    如果字段值有变化，删除老的service，创建所需的service，或者不创建service，取决于字段值

  2.3 delete:
    删除deployment时，也会删除子资源service
    删除子资源service时，如果对应的deployment仍存在，则维持service数目